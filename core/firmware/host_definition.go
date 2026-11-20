package firmware

import "errors"

func LoadHost(data []byte, reg Registry) (*Definition, error) {
	d, err := Parse(data)
	if err != nil {
		return nil, err
	}
	known, err := KnownSlugs()
	if err != nil {
		return nil, err
	}
	if errs := ValidateHost(d, known, reg); len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return d, nil
}
