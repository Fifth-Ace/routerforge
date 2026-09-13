package policy

import (
	"errors"
	"fmt"
	"strings"
)

type ObjectType string

const (
	Device      ObjectType = "device"
	Network     ObjectType = "network"
	DomainGroup ObjectType = "domain-group"
	Interface   ObjectType = "interface"
	Gateway     ObjectType = "gateway"
)

type Object struct {
	ID          string            `json:"id"`
	Type        ObjectType        `json:"type"`
	Name        string            `json:"name"`
	Source      string            `json:"source,omitempty"`
	Owner       string            `json:"owner,omitempty"`
	Description string            `json:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

func (o Object) Validate() error {
	id := strings.TrimSpace(o.ID)
	if id == "" || len(id) > 96 {
		return errors.New("policy object id must be 1..96 characters")
	}
	for _, r := range id {
		if !(r == '-' || r == '_' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return fmt.Errorf("policy object id contains unsupported character %q", r)
		}
	}
	switch o.Type {
	case Device, Network, DomainGroup, Interface, Gateway:
	default:
		return fmt.Errorf("unsupported policy object type %q", o.Type)
	}
	if strings.TrimSpace(o.Name) == "" || len(strings.TrimSpace(o.Name)) > 160 {
		return errors.New("policy object name must be 1..160 characters")
	}
	if len(o.Labels) > 32 {
		return errors.New("policy object labels exceed limit")
	}
	for key, value := range o.Labels {
		if len(key) > 64 || len(value) > 256 {
			return errors.New("policy object label exceeds size limit")
		}
	}
	return nil
}
