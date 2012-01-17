package charm

import (
	"fmt"
	"errors"
	"io"
	"io/ioutil"
	"launchpad.net/goyaml"
	"launchpad.net/juju/go/schema"
	"strconv"
)

// Option represents a single configuration option that is declared
// as supported by a charm in its config.yaml file.
type Option struct {
	Title       string
	Description string
	Type        string
	Default     interface{}
}

// Config represents the supported configuration options for a charm,
// as declared in its config.yaml file.
type Config struct {
	Options map[string]Option
}

// ReadConfig reads a config.yaml file and returns its representation.
func ReadConfig(r io.Reader) (config *Config, err error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return
	}
	raw := make(map[interface{}]interface{})
	err = goyaml.Unmarshal(data, raw)
	if err != nil {
		return
	}
	v, err := configSchema.Coerce(raw, nil)
	if err != nil {
		return nil, errors.New("config: " + err.Error())
	}
	config = &Config{}
	config.Options = make(map[string]Option)
	m := v.(schema.MapType)
	for name, infov := range m["options"].(schema.MapType) {
		opt := infov.(schema.MapType)
		optTitle, _ := opt["title"].(string)
		optType, _ := opt["type"].(string)
		optDescr, _ := opt["description"].(string)
		optDefault, _ := opt["default"]
		config.Options[name.(string)] = Option{
			Title:       optTitle,
			Type:        optType,
			Description: optDescr,
			Default:     optDefault,
		}
	}
	return
}

// Validate processes the values in the input map according to the
// configuration in config, doing the following operations:
//
// - Values are converted from strings to the types defined
// - Options with default values are introduced for missing keys
// - Unknown keys and badly typed values are reported as errors
// 
func (c *Config) Validate(values map[string]string) (processed map[string]interface{}, err error) {
	out := make(map[string]interface{})
	for k, v := range values {
		opt, ok := c.Options[k]
		if !ok {
			return nil, fmt.Errorf("Unknown configuration option: %q", k)
		}
		switch opt.Type {
		case "string":
			out[k] = v
		case "int":
			i, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Value for %q is not an int: %q", k, v)
			}
			out[k] = i
		case "float":
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, fmt.Errorf("Value for %q is not a float: %q", k, v)
			}
			out[k] = f
		default:
			panic(fmt.Errorf("Internal error: option type %q is unknown to Validate", opt.Type))
		}
	}
	for k, opt := range c.Options {
		if _, ok := out[k]; !ok && opt.Default != nil {
			out[k] = opt.Default
		}
	}
	return out, nil
}

var optionSchema = schema.FieldMap(
	schema.Fields{
		"type":        schema.OneOf(schema.Const("string"), schema.Const("int"), schema.Const("float")),
		"default":     schema.OneOf(schema.String(), schema.Int(), schema.Float()),
		"description": schema.String(),
	},
	schema.Optional{"default", "description"},
)

var configSchema = schema.FieldMap(
	schema.Fields{
		"options": schema.Map(schema.String(), optionSchema),
	},
	nil,
)