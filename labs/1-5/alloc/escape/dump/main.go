// Программа dump снимает решения escape-анализа для примеров из ../cases и печатает их в JSON —
// данные песочницы escape-lab (урок 1.5.8). Запуск из labs/:
//
//	go run ./1-5/alloc/escape/dump > ../course/viz/data/escape-lab.json
//
// Как строки компилятора сопоставляются с вариантами. Номера строк в выводе -gcflags=-m=1 указывают
// на выражение, а не на функцию, поэтому dump не ищет функцию по номеру строки напрямую: он разбирает
// исходники go/parser, находит функции с ключом «escape:<случай>/<вариант>» в комментарии и берёт
// их диапазоны строк. Строка компилятора относится к варианту, если попала в диапазон одной из его
// функций. У варианта может быть несколько функций: решение для //go:noinline-помощника компилятор
// печатает на строке самого помощника, поэтому ключ стоит и над ним. Встроенный код компилятор
// печатает на строке вызова — она внутри вызывающей функции.
package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const casesPkg = "labs/1-5/alloc/escape/cases"

// Record — одна запись JSON: вариант примера и решение компилятора для него.
type Record struct {
	Case    string `json:"case"`
	Variant string `json:"variant"`
	File    string `json:"file"`  // путь относительно alloc/escape/cases
	Line    int    `json:"line"`  // строка решающего сообщения компилятора
	Where   string `json:"where"` // "stack" или "heap"
	// Compiler — все строки -m=1 из функций варианта, кроме «can inline», через \n.
	Compiler string `json:"compiler"`
}

type span struct {
	file       string
	start, end int
}

type variant struct {
	caseName, name string
	spans          []span
	order          token.Position // позиция функции Case…: по ней сортируются записи
}

type diag struct {
	file      string
	line, col int
	msg       string
}

func (d diag) String() string { return fmt.Sprintf("%s:%d:%d: %s", d.file, d.line, d.col, d.msg) }

var (
	markerRe = regexp.MustCompile(`^// escape:([a-z0-9-]+)/([a-z0-9-]+)$`)
	diagRe   = regexp.MustCompile(`^(?:\./)?([^/\s:]+\.go):(\d+):(\d+): (.+)$`)
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "dump: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dir, err := packageDir(casesPkg)
	if err != nil {
		return err
	}
	variants, err := findVariants(dir)
	if err != nil {
		return err
	}
	diags, err := compilerDiags(dir)
	if err != nil {
		return err
	}
	records := make([]Record, 0, len(variants))
	for _, v := range variants {
		r, err := decide(v, diags)
		if err != nil {
			return err
		}
		records = append(records, r)
	}
	return write(os.Stdout, records)
}

func packageDir(pkg string) (string, error) {
	out, err := exec.Command("go", "list", "-f", "{{.Dir}}", pkg).Output()
	if err != nil {
		return "", fmt.Errorf("go list %s (запускайте из labs/): %w", pkg, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// findVariants собирает функции с ключом escape: и проверяет, что у каждого варианта ровно одна
// функция Case….
func findVariants(dir string) ([]*variant, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	byKey := map[string]*variant{}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("разбор %s: %w", path, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Doc == nil {
				continue
			}
			for _, c := range fn.Doc.List {
				m := markerRe.FindStringSubmatch(c.Text)
				if m == nil {
					continue
				}
				key := m[1] + "/" + m[2]
				v := byKey[key]
				if v == nil {
					v = &variant{caseName: m[1], name: m[2]}
					byKey[key] = v
				}
				pos, end := fset.Position(fn.Pos()), fset.Position(fn.End())
				v.spans = append(v.spans, span{file: filepath.Base(path), start: pos.Line, end: end.Line})
				if strings.HasPrefix(fn.Name.Name, "Case") {
					if v.order.IsValid() {
						return nil, fmt.Errorf("у варианта %s две функции Case…", key)
					}
					v.order = pos
				}
			}
		}
	}
	if len(byKey) == 0 {
		return nil, fmt.Errorf("в %s нет ни одного ключа «escape:<случай>/<вариант>»", dir)
	}
	variants := make([]*variant, 0, len(byKey))
	for key, v := range byKey {
		if !v.order.IsValid() {
			return nil, fmt.Errorf("у варианта %s нет функции Case…", key)
		}
		variants = append(variants, v)
	}
	slices.SortStableFunc(variants, func(a, b *variant) int {
		return cmp.Or(cmp.Compare(a.order.Filename, b.order.Filename), cmp.Compare(a.order.Line, b.order.Line))
	})
	return variants, nil
}

// compilerDiags компилирует пакет с -gcflags=-m=1. Кэш сборки хранит вывод компилятора и
// воспроизводит его, поэтому повторный запуск не пересобирает пакет и печатает то же самое.
func compilerDiags(dir string) ([]diag, error) {
	cmd := exec.Command("go", "build", "-gcflags=-m=1", "-o", os.DevNull, ".")
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go build -gcflags=-m=1: %w\n%s", err, out.String())
	}
	var diags []diag
	for line := range strings.SplitSeq(out.String(), "\n") {
		m := diagRe.FindStringSubmatch(line)
		if m == nil {
			continue // заголовок «# пакет» и пустые строки
		}
		ln, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		diags = append(diags, diag{file: m[1], line: ln, col: col, msg: m[4]})
	}
	if len(diags) == 0 {
		return nil, errors.New("компилятор не напечатал ни одной строки -m")
	}
	return diags, nil
}

// allocStays — строка «<выражение> does not escape» о выделении памяти (new, make, &T{}, замыкание),
// а не о параметре функции: «ch does not escape» говорит о том, что сохраняет функция, а не где лежит
// её значение.
func allocStays(d diag) bool {
	subject, ok := strings.CutSuffix(d.msg, " does not escape")
	return ok && !token.IsIdentifier(subject)
}

// decide собирает строки компилятора из функций варианта. Вариант — «heap», если хоть одна строка
// говорит, что значение убегает («escapes to heap») или переменная переносится в кучу («moved to
// heap»); решающая строка — первая такая или, для стека, первая строка allocStays.
func decide(v *variant, diags []diag) (Record, error) {
	var own []diag
	for _, d := range diags {
		if strings.HasPrefix(d.msg, "can inline ") {
			continue // о встраиваемости самой функции, а не о месте значений
		}
		if slices.ContainsFunc(v.spans, func(s span) bool { return s.file == d.file && d.line >= s.start && d.line <= s.end }) {
			own = append(own, d)
		}
	}
	slices.SortStableFunc(own, func(a, b diag) int {
		return cmp.Or(cmp.Compare(a.file, b.file), cmp.Compare(a.line, b.line), cmp.Compare(a.col, b.col))
	})
	key := v.caseName + "/" + v.name
	heap := func(d diag) bool {
		return strings.Contains(d.msg, "escapes to heap") || strings.HasPrefix(d.msg, "moved to heap")
	}
	where := "heap"
	i := slices.IndexFunc(own, heap)
	if i < 0 {
		where = "stack"
		i = slices.IndexFunc(own, allocStays)
	}
	if i < 0 {
		return Record{}, fmt.Errorf("у варианта %s нет решающей строки компилятора: %v", key, own)
	}
	lines := make([]string, len(own))
	for j, d := range own {
		lines[j] = d.String()
	}
	return Record{
		Case:     v.caseName,
		Variant:  v.name,
		File:     own[i].file,
		Line:     own[i].line,
		Where:    where,
		Compiler: strings.Join(lines, "\n"),
	}, nil
}

// write печатает массив JSON по записи на строку: так файл удобно сравнивать в diff и проверять grep.
func write(w io.Writer, records []Record) error {
	var buf bytes.Buffer
	buf.WriteString("[\n")
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // «&point{...}» остаётся читаемым, без &
	for i, r := range records {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("кодирование %s/%s: %w", r.Case, r.Variant, err)
		}
		if i < len(records)-1 {
			buf.Truncate(buf.Len() - 1) // Encode завершает запись переводом строки
			buf.WriteString(",\n")
		}
	}
	buf.WriteString("]\n")
	_, err := w.Write(buf.Bytes())
	return err
}
