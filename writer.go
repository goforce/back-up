package main

import (
	"encoding/csv"
	"fmt"
	"github.com/goforce/api/conv"
	"github.com/goforce/api/soap"
	"os"
)

type Writer struct {
	file   *os.File
	writer *csv.Writer
	fields []string
}

func NewWriter(path string, fields []string) *Writer {
	var err error
	w := &Writer{}
	w.file, err = os.Create(path)
	if err != nil {
		panic(fmt.Sprint("failed to create file:", path, "\n", err))
	}
	w.writer = csv.NewWriter(w.file)
	w.fields = fields
	if err := w.writer.Write(w.fields); err != nil {
		panic(fmt.Sprint("write to file failed:", path, "\n", err))
	}
	return w
}

func (w *Writer) Write(rec soap.Record) {
	var err error
	row := make([]string, len(w.fields))
	for i, name := range w.fields {
		if value, ok := rec.Get(name); ok {
			row[i], err = conv.String(value)
			if err != nil {
				panic(fmt.Sprint("failed to convert value:", value, " for field:", name, " error:", err))
			}
		}
	}
	if err = w.writer.Write(row); err != nil {
		panic(fmt.Sprint("write to file failed:", err))
	}
}

func (w *Writer) Close() {
	w.writer.Flush()
	if err := w.writer.Error(); err != nil {
		panic(fmt.Sprint("failed to flush file:", err))
	}
	if err := w.file.Close(); err != nil {
		panic(fmt.Sprint("failed to close file: ", err))
	}
}
