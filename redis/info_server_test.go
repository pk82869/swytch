/*
 * Copyright 2026 Swytch Labs BV
 *
 * This file is part of Swytch.
 *
 * Swytch is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of
 * the License, or (at your option) any later version.
 *
 * Swytch is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with Swytch. If not, see <https://www.gnu.org/licenses/>.
 */
package redis

import (
	"strings"
	"testing"
)

// setVersion overrides the build-time Version for one test.
func setVersion(t *testing.T, v string) {
	t.Helper()

	prev := Version
	Version = v

	t.Cleanup(func() { Version = prev })
}

// infoFields sends INFO with the given arguments and returns the reply's
// field:value lines keyed by field.
func infoFields(t *testing.T, c *redisClient, args ...string) (string, map[string]string) {
	t.Helper()

	c.sendCommand(append([]string{"INFO"}, args...)...)
	resp := c.readReply()

	fields := make(map[string]string)

	for line := range strings.SplitSeq(resp, "\r\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && !strings.HasPrefix(line, "#") {
			fields[k] = v
		}
	}

	return resp, fields
}

func TestInfoServerReportsSeparateVersions(t *testing.T) {
	const appVersion = "0.0.0-infotest"

	setVersion(t, appVersion)

	addr, cleanup := startRedisTestServer(t)
	defer cleanup()

	c := newRedisClient(t, addr)
	defer c.close()

	for _, args := range [][]string{{"server"}, {"SERVER"}, {}, {"all"}} {
		resp, fields := infoFields(t, c, args...)

		if !strings.Contains(resp, "# Server\r\n") {
			t.Fatalf("INFO %v: missing Server section: %q", args, resp)
		}

		want := map[string]string{
			"redis_version":   redisCompatVersion,
			"server_name":     "swytch",
			"swytch_version":  appVersion,
			"redis_git_sha1":  "00000000",
			"redis_git_dirty": "0",
			"redis_build_id":  "swytch",
			"redis_mode":      "standalone",
		}
		for k, v := range want {
			if got := fields[k]; got != v {
				t.Errorf("INFO %v: %s = %q, want %q", args, k, got, v)
			}
		}

		if strings.Contains(fields["redis_version"], appVersion) || strings.Contains(fields["redis_version"], "swytch") {
			t.Errorf("INFO %v: redis_version %q mixes in the Swytch version", args, fields["redis_version"])
		}

		for _, k := range []string{"os", "arch_bits", "process_id", "tcp_port"} {
			if _, ok := fields[k]; !ok {
				t.Errorf("INFO %v: missing %s", args, k)
			}
		}
	}
}

func TestInfoServerDevBuildVersion(t *testing.T) {
	setVersion(t, "dev")

	h := NewHandler(DefaultHandlerConfig())
	defer h.Close()

	info := h.buildInfoString([][]byte{[]byte("server")})

	if !strings.Contains(info, "\r\nswytch_version:dev\r\n") {
		t.Fatalf("dev build: swytch_version not reported as dev: %q", info)
	}

	if !strings.Contains(info, "\r\nredis_version:"+redisCompatVersion+"\r\n") {
		t.Fatalf("dev build: redis_version not %s: %q", redisCompatVersion, info)
	}
}

func TestInfoOtherSectionsOmitServerFields(t *testing.T) {
	setVersion(t, "0.0.0-infotest")

	addr, cleanup := startRedisTestServer(t)
	defer cleanup()

	c := newRedisClient(t, addr)
	defer c.close()

	for _, section := range []string{"clients", "memory", "stats"} {
		resp, fields := infoFields(t, c, section)

		if strings.Contains(resp, "# Server") {
			t.Errorf("INFO %s: unexpected Server section: %q", section, resp)
		}

		for _, k := range []string{"redis_version", "server_name", "swytch_version"} {
			if _, ok := fields[k]; ok {
				t.Errorf("INFO %s: unexpected %s", section, k)
			}
		}
	}
}

func TestHelloReportsCompatVersion(t *testing.T) {
	setVersion(t, "0.0.0-infotest")

	addr, cleanup := startRedisTestServer(t)
	defer cleanup()

	c := newRedisClient(t, addr)
	defer c.close()

	c.sendCommand("HELLO")

	if got, want := c.readReply(), "server,swytch,version,"+redisCompatVersion+","; !strings.HasPrefix(got, want) {
		t.Fatalf("HELLO = %q, want prefix %q", got, want)
	}
}
