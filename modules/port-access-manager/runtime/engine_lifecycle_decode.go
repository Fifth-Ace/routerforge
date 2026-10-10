package main

import (
	"encoding/json"
	"errors"
	"io"
)

func decodeEngineAction(body io.Reader, out any) error {
	d := json.NewDecoder(body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing data")
	}
	return nil
}
