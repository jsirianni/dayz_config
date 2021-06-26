package kamyshowdown

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type Param struct {
	raw     string
	key     string
	value   string
	semi    string
	comment string
}

/**

Enforce syntax like so: 'key = value; // comment

Space between key and '='
Space between '=' and value
Semicolon follows value
Space between ; and comment
Comment is prefixed with //
Values can be quoted
Values can be strings or numbers

Example:

hostname = "Kamyshowdown | Vanilla | DogShitGames.com";  // Server name
password = "";              // Password to connect to the server, do not store in git
disableCrosshair = 1;         // Toggles the cross-hair (value 0-1)

**/
func TestServerCFG(t *testing.T) {
	r := regexp.MustCompile(`^(?P<param>[\w\-]+) = (?P<value>.*)(?P<semi>;) (?P<comment>.*)`)

	foundHostname := false // will be true if hostname is validated at bottom

	xmlFile, err := os.Open("../serverDZ.cfg")
	if err != nil {
		require.NoError(t, err)
	}
	defer xmlFile.Close()

	scanner := bufio.NewScanner(xmlFile)
	for scanner.Scan() {
		in := scanner.Text()

		// Skip blank lines
		if strings.TrimSpace(in) == "" {
			continue
		}

		// Exit after checking all parameters
		if strings.Contains(in, "class Missions") {
			break
		}

		x := r.FindStringSubmatch(in)
		l := len(x)
		if l < 4 || l > 5 {
			t.Errorf("expected regex result to return 4 or 5 values, got %d.\nRaw string: '%s'", l, in)
			return
		}

		param := Param{
			raw:   x[0],
			key:   x[1],
			value: x[2],
			semi:  x[3],
		}
		if l == 5 {
			param.comment = x[4]
		}

		require.NotEmpty(t, param.key, fmt.Sprintf("parameter name should not be empty.\nRaw string: '%s'", in))
		require.NotEmpty(t, param.value, fmt.Sprintf("parameter %s value should not be empty.\nRaw string: '%s'", param.key, in))
		require.Equal(t, ";", param.semi, fmt.Sprintf("expected semi colon after value and before comment.\nRaw string: '%s'", in))
		if len(param.comment) > 0 {
			s := strings.TrimSpace(param.comment)
			require.True(t, strings.HasPrefix(s, "//"), fmt.Sprintf("comments should be prefixed with //, got '%s'.\nRaw string: '%s'", s, in))
		}

		if param.key == "hostname" {
			require.Equal(t, `"Kamyshowdown | Vanilla | DogShitGames.com"`, param.value, "server name is not as expected")
			foundHostname = true
		}
	}

	if err := scanner.Err(); err != nil {
		require.NoError(t, err, "error scanning file")
	}

	require.True(t, foundHostname, "expected 'hostname' param to be in config file")
}
