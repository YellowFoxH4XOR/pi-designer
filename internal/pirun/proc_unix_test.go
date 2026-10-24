//go:build unix

package pirun

import (
	"bufio"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestKillProcessGroupReapsDescendantsAfterLeaderExit(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 30 & echo $!")
	setProcessGroup(cmd)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	child, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Kill(child, syscall.SIGKILL)
	_ = cmd.Wait()

	killProcessGroup(cmd)
	deadline := time.Now().Add(2 * time.Second)
	for syscall.Kill(child, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatal("background descendant survived its exited group leader")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
