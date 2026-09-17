package main

import (
	"bytes"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

func snapshotMetrics(r *prometheus.Registry) ([]byte, error) {
	families, err := r.Gather()
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	encoder := expfmt.NewEncoder(&buffer, expfmt.NewFormat(expfmt.TypeTextPlain))
	for _, family := range families {
		if err := encoder.Encode(family); err != nil {
			return nil, err
		}
	}
	return buffer.Bytes(), nil
}
