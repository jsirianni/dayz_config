package deploy

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// AuthFrame is a bounded, ephemeral stream. Answers must never be persisted.
type AuthFrame struct {
	Version int    `json:"version"`
	Type    string `json:"type"`
	Value   string `json:"value,omitempty"`
}

func readAuthFrame(r *bufio.Reader) (AuthFrame, error) {
	var f AuthFrame
	line, err := r.ReadSlice('\n')
	if err != nil {
		return f, err
	}
	if len(line) > 8192 {
		return f, errors.New("authentication frame too large")
	}
	if err := json.Unmarshal(line, &f); err != nil {
		return f, errors.New("invalid authentication frame")
	}
	if f.Version != Protocol {
		return f, VersionError(f.Version)
	}
	return f, nil
}
func sendAuth(w io.Writer, kind, value string) error {
	return json.NewEncoder(w).Encode(AuthFrame{Version: Protocol, Type: kind, Value: value})
}

func authConversation(out io.Reader, in io.Writer, prompt func(AuthFrame) (string, error)) error {
	r := bufio.NewReaderSize(out, 8192)
	for {
		f, err := readAuthFrame(r)
		if err != nil {
			return fmt.Errorf("Steam authentication stream ended before success: %w", err)
		}
		switch f.Type {
		case "ready":
			return nil
		case "error":
			return errors.New("Steam authentication failed; rerun deploy to retry authentication")
		case "password", "guard":
			if prompt == nil {
				return errors.New("Steam authentication requires an interactive terminal")
			}
			answer, err := prompt(f)
			if err != nil {
				return err
			}
			if answer == "" || len(answer) > 4096 || strings.ContainsAny(answer, "\r\n\x00") {
				return errors.New("invalid Steam authentication response")
			}
			if err = sendAuth(in, "answer", answer); err != nil {
				return err
			}
		case "approval", "status":
			if prompt != nil {
				if _, err = prompt(f); err != nil {
					return err
				}
			}
		default:
			return errors.New("unsupported authentication message; upgrade client and agent together")
		}
	}
}

func ReadSecret(message string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("authentication needs an interactive terminal; run dzdeploy directly in a terminal")
	}
	fmt.Fprint(os.Stderr, message)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	defer clear(b)
	return string(b), err
}

// ReadInput collects non-secret setup values inside the deployment command.
func ReadInput(message string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("setup needs an interactive terminal; run dzdeploy directly in a terminal")
	}
	fmt.Fprint(os.Stderr, message)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line), err
}

// SteamDialog recognizes prompts even when SteamCMD doesn't end them with a
// newline. Raw output is deliberately not returned to the client or a log.
type steamDialog struct {
	tail     string
	success  bool
	approval bool
}

func (d *steamDialog) consume(chunk string) string {
	d.tail += strings.ToLower(chunk)
	if len(d.tail) > 8192 {
		d.tail = d.tail[len(d.tail)-8192:]
	}
	if strings.Contains(d.tail, "logged in ok") || strings.Contains(d.tail, "waiting for user info...ok") || strings.Contains(d.tail, "waiting for user info... ok") {
		d.success = true
	}
	for _, p := range []string{"password:", "password: ", "enter password"} {
		if strings.Contains(d.tail, p) {
			d.tail = ""
			return "password"
		}
	}
	for _, p := range []string{"steam guard code:", "steamguard code:", "two-factor code:", "two-factor authentication code:", "code from your email:", "steam guard code is required", "enter the current code", "enter the steam guard code"} {
		if strings.Contains(d.tail, p) {
			d.tail = ""
			return "guard"
		}
	}
	if !d.approval && (strings.Contains(d.tail, "waiting for confirmation") || strings.Contains(d.tail, "confirm the login") || strings.Contains(d.tail, "approve this sign in") || strings.Contains(d.tail, "steam mobile app")) {
		d.approval = true
		d.tail = ""
		return "approval"
	}
	return ""
}
