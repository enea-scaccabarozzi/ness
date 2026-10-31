package config

import "github.com/BurntSushi/toml"

// Head is the part of a ness.toml that another ness binary must still be able to read: the pin,
// the project name and the [update] table (R-UPD-046).
type Head struct {
	Ness    string `toml:"ness"`
	Project string `toml:"project"`
	Update  Update `toml:"update"`
}

// ReadHead decodes only ness, project and [update] from data, ignoring every other key (the file
// may belong to a newer binary), with the Update defaults applied. Values are not validated.
func ReadHead(data []byte) (Head, error) {
	h := Head{Update: defaultUpdate()}
	if _, err := toml.Decode(string(data), &h); err != nil {
		return Head{}, posError(err)
	}
	return h, nil
}
