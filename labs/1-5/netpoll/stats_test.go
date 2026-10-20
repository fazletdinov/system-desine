package main

import "testing"

func TestParseVmRSS(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		want    int
		wantErr bool
	}{
		{
			name:   "строка среди прочих",
			status: "Name:\tnetpoll\nVmPeak:\t  812345 kB\nVmRSS:\t   10240 kB\nThreads:\t9\n",
			want:   10240,
		},
		{name: "нет строки", status: "Name:\tnetpoll\nThreads:\t9\n", wantErr: true},
		{name: "не число", status: "VmRSS:\t abc kB\n", wantErr: true},
		{name: "другая единица", status: "VmRSS:\t 10 MB\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVmRSS(tt.status)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseVmRSS() ошибка %v, ждали ошибку: %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("parseVmRSS() = %d, ждали %d", got, tt.want)
			}
		})
	}
}

func TestParseSockstatTCP(t *testing.T) {
	const sockstat = "sockets: used 2203\n" +
		"TCP: inuse 2010 orphan 0 tw 3 alloc 2015 mem 12\n" +
		"UDP: inuse 4 mem 2\n"
	got, err := parseSockstatTCP(sockstat)
	if err != nil {
		t.Fatalf("parseSockstatTCP(): %v", err)
	}
	if got.inUse != 2010 || got.mem != 12 {
		t.Fatalf("inuse %d, mem %d; ждали 2010 и 12", got.inUse, got.mem)
	}
	if got.line != "TCP: inuse 2010 orphan 0 tw 3 alloc 2015 mem 12" {
		t.Fatalf("строка %q", got.line)
	}

	for name, text := range map[string]string{
		"нет строки TCP":  "sockets: used 1\nUDP: inuse 4 mem 2\n",
		"нет поля mem":    "TCP: inuse 1 orphan 0 tw 0 alloc 1\n",
		"не число в поле": "TCP: inuse x orphan 0 tw 0 alloc 1 mem 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseSockstatTCP(text); err == nil {
				t.Fatalf("parseSockstatTCP(%q) без ошибки", text)
			}
		})
	}
}
