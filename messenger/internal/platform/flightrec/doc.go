// Package flightrec держит flight recorder (runtime/trace.FlightRecorder) — скользящее
// окно трассы выполнения последних секунд — и пишет его снимок в файл, когда запрос
// оказался медленнее порога (урок 1.5.10). Рекордер один на процесс: его запускает
// composition root, а Dumper и служебный порт получают узкий порт WriteTo.
package flightrec
