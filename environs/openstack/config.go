// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package openstack

import (
	"fmt"
	"launchpad.net/goose/identity"
	"launchpad.net/juju-core/environs/config"
	"launchpad.net/juju-core/log"
	"launchpad.net/juju-core/schema"
	"net/url"
)

var configFields = schema.Fields{
	"username":          schema.String(),
	"password":          schema.String(),
	"tenant-name":       schema.String(),
	"auth-url":          schema.String(),
	"auth-mode":         schema.String(),
	"access-key":        schema.String(),
	"secret-key":        schema.String(),
	"region":            schema.String(),
	"control-bucket":    schema.String(),
	"public-bucket":     schema.String(),
	"public-bucket-url": schema.String(),
	"use-floating-ip":   schema.Bool(),
	// These next keys are deprecated and ignored. We keep them them in the schema
	// so existing configs do not error.
	"default-image-id":      schema.String(),
	"default-instance-type": schema.String(),
}
var configDefaults = schema.Defaults{
	"username":          "",
	"password":          "",
	"tenant-name":       "",
	"auth-url":          "",
	"auth-mode":         string(AuthUserPass),
	"access-key":        "",
	"secret-key":        "",
	"region":            "",
	"control-bucket":    "",
	"public-bucket":     "juju-dist",
	"public-bucket-url": "",
	"use-floating-ip":   false,
	// These next keys are deprecated and ignored. We keep them them in the schema
	// so existing configs do not error.
	"default-image-id":      "",
	"default-instance-type": "",
}

type environConfig struct {
	*config.Config
	attrs map[string]interface{}
}

func (c *environConfig) region() string {
	return c.attrs["region"].(string)
}

func (c *environConfig) username() string {
	return c.attrs["username"].(string)
}

func (c *environConfig) password() string {
	return c.attrs["password"].(string)
}

func (c *environConfig) tenantName() string {
	return c.attrs["tenant-name"].(string)
}

func (c *environConfig) authURL() string {
	return c.attrs["auth-url"].(string)
}

func (c *environConfig) authMode() string {
	return c.attrs["auth-mode"].(string)
}

func (c *environConfig) accessKey() string {
	return c.attrs["access-key"].(string)
}

func (c *environConfig) secretKey() string {
	return c.attrs["secret-key"].(string)
}

func (c *environConfig) controlBucket() string {
	return c.attrs["control-bucket"].(string)
}

func (c *environConfig) publicBucket() string {
	return c.attrs["public-bucket"].(string)
}

func (c *environConfig) publicBucketURL() string {
	return c.attrs["public-bucket-url"].(string)
}

func (c *environConfig) useFloatingIP() bool {
	return c.attrs["use-floating-ip"].(bool)
}

func (p environProvider) newConfig(cfg *config.Config) (*environConfig, error) {
	valid, err := p.Validate(cfg, nil)
	if err != nil {
		return nil, err
	}
	return &environConfig{valid, valid.UnknownAttrs()}, nil
}

type AuthMode string

const (
	AuthKeyPair  AuthMode = "keypair"
	AuthLegacy   AuthMode = "legacy"
	AuthUserPass AuthMode = "userpass"
)

func (p environProvider) Validate(cfg, old *config.Config) (valid *config.Config, err error) {
	// Check for valid changes for the base config values.
	if err := config.Validate(cfg, old); err != nil {
		return nil, err
	}

	// Check for deprecated fields and log a warning. We also print to stderr to ensure the user sees the message
	// even if they are not running with --debug.
	if defaultImageId := cfg.AllAttrs()["default-image-id"]; defaultImageId != nil && defaultImageId.(string) != "" {
		msg := fmt.Sprintf(
			"config attribute %q (%v) is deprecated and ignored, use simplestreams metadata instead",
			"default-image-id", defaultImageId)
		log.Warningf(msg)
	}
	if defaultInstanceType := cfg.AllAttrs()["default-instance-type"]; defaultInstanceType != nil && defaultInstanceType.(string) != "" {
		msg := fmt.Sprintf(
			"config attribute %q (%v) is deprecated and ignored", "default-instance-type", defaultInstanceType)
		log.Warningf(msg)
	}

	validated, err := cfg.ValidateUnknownAttrs(configFields, configDefaults)
	if err != nil {
		return nil, err
	}
	ecfg := &environConfig{cfg, validated}

	authMode := AuthMode(ecfg.authMode())
	switch authMode {
	case AuthKeyPair:
	case AuthLegacy:
	case AuthUserPass:
	default:
		return nil, fmt.Errorf("invalid authorization mode: %q", authMode)
	}

	if ecfg.authURL() != "" {
		parts, err := url.Parse(ecfg.authURL())
		if err != nil || parts.Host == "" || parts.Scheme == "" {
			return nil, fmt.Errorf("invalid auth-url value %q", ecfg.authURL())
		}
	}
	cred := identity.CredentialsFromEnv()
	format := "required environment variable not set for credentials attribute: %s"
	if authMode == AuthUserPass || authMode == AuthLegacy {
		if ecfg.username() == "" {
			if cred.User == "" {
				return nil, fmt.Errorf(format, "User")
			}
			ecfg.attrs["username"] = cred.User
		}
		if ecfg.password() == "" {
			if cred.Secrets == "" {
				return nil, fmt.Errorf(format, "Secrets")
			}
			ecfg.attrs["password"] = cred.Secrets
		}
	} else if authMode == AuthKeyPair {
		if ecfg.accessKey() == "" {
			if cred.User == "" {
				return nil, fmt.Errorf(format, "User")
			}
			ecfg.attrs["access-key"] = cred.User
		}
		if ecfg.secretKey() == "" {
			if cred.Secrets == "" {
				return nil, fmt.Errorf(format, "Secrets")
			}
			ecfg.attrs["secret-key"] = cred.Secrets
		}
	}
	if ecfg.authURL() == "" {
		if cred.URL == "" {
			return nil, fmt.Errorf(format, "URL")
		}
		ecfg.attrs["auth-url"] = cred.URL
	}
	if ecfg.tenantName() == "" {
		if cred.TenantName == "" {
			return nil, fmt.Errorf(format, "TenantName")
		}
		ecfg.attrs["tenant-name"] = cred.TenantName
	}
	if ecfg.region() == "" {
		if cred.Region == "" {
			return nil, fmt.Errorf(format, "Region")
		}
		ecfg.attrs["region"] = cred.Region
	}

	if old != nil {
		attrs := old.UnknownAttrs()
		if region, _ := attrs["region"].(string); ecfg.region() != region {
			return nil, fmt.Errorf("cannot change region from %q to %q", region, ecfg.region())
		}
		if controlBucket, _ := attrs["control-bucket"].(string); ecfg.controlBucket() != controlBucket {
			return nil, fmt.Errorf("cannot change control-bucket from %q to %q", controlBucket, ecfg.controlBucket())
		}
	}

	// Apply the coerced unknown values back into the config.
	return cfg.Apply(ecfg.attrs)
}
