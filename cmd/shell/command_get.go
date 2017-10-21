package shell

import (
	"bytes"
	"github.com/chrislusf/vasto/cmd/client"
)

func init() {
	commands = append(commands, &CommandGet{})
}

type CommandGet struct {
	client *client.VastoClient
}

func (c *CommandGet) Name() string {
	return "get"
}

func (c *CommandGet) Help() string {
	return "key"
}

func (c *CommandGet) SetCilent(client *client.VastoClient) {
	c.client = client
}

func (c *CommandGet) Do(args []string) (string, error) {
	if len(args) == 1 {
		key := []byte(args[0])

		value, err := c.client.Get(key)

		return string(value) + "\n", err
	} else {
		var keys [][]byte
		for _, arg := range args {
			keys = append(keys, []byte(arg))
		}
		keyValues, err := c.client.BatchGet(keys...)
		if err != nil {
			return "", err
		}
		var output bytes.Buffer
		for _, keyValue := range keyValues {
			output.Write(keyValue.Key)
			output.WriteString(" : ")
			output.Write(keyValue.Value)
			output.WriteString("\n")
		}
		return output.String(), nil
	}
}
