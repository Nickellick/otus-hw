package hw10programoptimization

import (
	"archive/zip"
	"testing"
)

func BenchmarkGetDomainStat(b *testing.B) {
	archive, err := zip.OpenReader("testdata/users.dat.zip")
	if err != nil {
		b.Fatal(err)
	}
	defer archive.Close()

	if len(archive.File) != 1 {
		b.Fatalf("expected one file, got %d", len(archive.File))
	}

	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()

		data, err := archive.File[0].Open()
		if err != nil {
			b.Fatal(err)
		}

		b.StartTimer()

		_, err = GetDomainStat(data, "biz")

		b.StopTimer()

		if err != nil {
			data.Close()
			b.Fatal(err)
		}

		if err := data.Close(); err != nil {
			b.Fatal(err)
		}
	}
}
