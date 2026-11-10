package masquerade

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// Mode represents the rendering stealth mode.
type Mode int

const (
	ModeClassic Mode = iota
	ModeLog
	ModeCode
	ModeStatusLine
)

var modeNames = []string{
	"经典极简 (Classic)",
	"微服务日志流 (Log Stream)",
	"源码注释 (Code Comment)",
	"单行状态栏 (Status Line)",
}

// Name returns the readable name of the mode.
func (m Mode) Name() string {
	if int(m) >= 0 && int(m) < len(modeNames) {
		return modeNames[m]
	}
	return "未知模式"
}

// NextMode cycles to the next stealth mode.
func (m Mode) Next() Mode {
	return (m + 1) % Mode(len(modeNames))
}

var logPackages = []string{
	"c.b.s.order.OrderWorker",
	"c.b.s.payment.PaymentEngine",
	"c.b.s.dispatch.TaskDispatcher",
	"c.b.s.storage.ShardManager",
	"c.b.s.gateway.RouterFilter",
}

var logLevels = []string{"INFO ", "INFO ", "INFO ", "DEBUG", "WARN "}

// FormatLog wraps novel paragraphs as enterprise microservice logs.
func FormatLog(paragraphs []string, start time.Time) string {
	if len(paragraphs) == 0 {
		return ""
	}

	var b strings.Builder
	currTime := start
	r := rand.New(rand.NewSource(start.UnixNano()))

	for i, p := range paragraphs {
		// Advance timestamp slightly between 1.2s - 3.5s
		currTime = currTime.Add(time.Duration(1200+r.Intn(2300)) * time.Millisecond)
		timeStr := currTime.Format("2006-01-02 15:04:05.000")
		workerID := r.Intn(16) + 1
		pkg := logPackages[r.Intn(len(logPackages))]
		lvl := logLevels[r.Intn(len(logLevels))]

		line := fmt.Sprintf("%s [%s] [worker-pool-%02d] %s - %s\n",
			timeStr, lvl, workerID, pkg, p)
		b.WriteString(line)

		// Every few paragraphs, insert an extra fake system debug log to increase authenticity
		if i%4 == 3 {
			currTime = currTime.Add(150 * time.Millisecond)
			fakeHeartbeat := fmt.Sprintf("%s [DEBUG] [worker-pool-%02d] %s - sync buffer snapshot, batch_id=%x, pending=0\n",
				currTime.Format("2006-01-02 15:04:05.000"), workerID, pkg, r.Int31())
			b.WriteString(fakeHeartbeat)
		}
	}

	return b.String()
}

// FormatCode wraps paragraphs inside code comments of a simulated Go source file.
func FormatCode(paragraphs []string, pkgName string) string {
	if pkgName == "" {
		pkgName = "service"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("package %s\n\n", pkgName))
	b.WriteString("import (\n\t\"context\"\n\t\"time\"\n)\n\n")

	for i, p := range paragraphs {
		fnName := fmt.Sprintf("ExecuteTask%03d", i+1)
		b.WriteString(fmt.Sprintf("// %s\n", p))
		b.WriteString(fmt.Sprintf("// @TraceID: req-%06d\n", i*107+42))
		b.WriteString(fmt.Sprintf("func (s *Handler) %s(ctx context.Context) error {\n", fnName))
		b.WriteString("\tselect {\n")
		b.WriteString("\tcase <-ctx.Done():\n")
		b.WriteString("\t\treturn ctx.Err()\n")
		b.WriteString("\tdefault:\n")
		b.WriteString("\t\ttime.Sleep(10 * time.Millisecond)\n")
		b.WriteString("\t\treturn nil\n")
		b.WriteString("\t}\n")
		b.WriteString("}\n\n")
	}

	return b.String()
}

// FormatClassic wraps paragraphs with clean indentations and double linebreaks.
func FormatClassic(paragraphs []string) string {
	if len(paragraphs) == 0 {
		return ""
	}
	var b strings.Builder
	for i, p := range paragraphs {
		b.WriteString("  ")
		b.WriteString(p)
		if i < len(paragraphs)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

// GenerateFakeGoTestScreen returns a convincing stream of `go test -v ./...` output for the boss key.
func GenerateFakeGoTestScreen() string {
	return `=== RUN   TestClusterRaftElection
=== RUN   TestClusterRaftElection/single_node
=== RUN   TestClusterRaftElection/leader_failover
--- PASS: TestClusterRaftElection (0.42s)
    --- PASS: TestClusterRaftElection/single_node (0.12s)
    --- PASS: TestClusterRaftElection/leader_failover (0.30s)
=== RUN   TestStorageEngineWriteAheadLog
=== RUN   TestStorageEngineWriteAheadLog/wal_replay
=== RUN   TestStorageEngineWriteAheadLog/checkpoint_sync
--- PASS: TestStorageEngineWriteAheadLog (1.28s)
    --- PASS: TestStorageEngineWriteAheadLog/wal_replay (0.64s)
    --- PASS: TestStorageEngineWriteAheadLog/checkpoint_sync (0.64s)
=== RUN   TestPaymentIdempotencyDispatch
=== RUN   TestPaymentIdempotencyDispatch/concurrent_requests
=== RUN   TestPaymentIdempotencyDispatch/distributed_lock_timeout
--- PASS: TestPaymentIdempotencyDispatch (0.88s)
    --- PASS: TestPaymentIdempotencyDispatch/concurrent_requests (0.50s)
    --- PASS: TestPaymentIdempotencyDispatch/distributed_lock_timeout (0.38s)
=== RUN   TestGRPCGatewayRouteTable
--- PASS: TestGRPCGatewayRouteTable (0.05s)
PASS
coverage: 88.4% of statements in ./...
ok      internal/core/cluster     2.63s
`
}

// GenerateFakeHtopScreen returns a static ASCII mockup of htop.
func GenerateFakeHtopScreen() string {
	return `  1  [|||||||||||||||||||||||||                    42.8%]   Tasks: 184, 912 thr; 1 running
  2  [|||||||||||||                                26.1%]   Load average: 1.48 1.62 1.55 
  3  [||||||||||||||||||||                         34.0%]   Uptime: 14 days, 06:12:44
  4  [||||||||||                                   18.5%]
  Mem[|||||||||||||||||||||||||||||||||     14.2G/32.0G]
  Swp[|                                      256M/4.00G]

  PID USER      PRI  NI  VIRT   RES   SHR S CPU% MEM%   TIME+  Command
 4192 dev        20   0 18.2G  2.4G 48200 S 24.5  7.5  8:41.22 /usr/local/bin/dockerd
 8491 dev        20   0 4210M  820M 32100 S 14.2  2.5  3:18.40 go run ./cmd/server
 9201 dev        20   0 2140M  412M 18900 S  8.5  1.2  1:02.15 redis-server *:6379
 1042 dev        20   0  812M  142M  8400 S  2.1  0.4  0:14.88 /usr/sbin/sshd -D
 7412 dev        20   0 1480M  288M 12400 S  1.5  0.9  0:42.10 prometheus --config.file
`
}

// GenerateFakeGitScreen returns an authentic-looking git status & git log screen.
func GenerateFakeGitScreen() string {
	return `On branch feature/payment-idempotency-v2
Your branch is up to date with 'origin/feature/payment-idempotency-v2'.

Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git restore <file>..." to discard changes in working directory)
	modified:   internal/handler/order_dispatch.go
	modified:   internal/service/idempotency_checker.go

Untracked files:
  (use "git add <file>..." to include in what will be committed)
	internal/service/idempotency_checker_test.go

no changes added to commit (use "git add" to track)

* 7f9a20d (HEAD -> feature/payment-idempotency-v2) refactor: optimize lock acquisition timeout
* 3b1c8e1 fix(storage): prevent deadlocks on concurrent shard flush
* e4d01aa test: add distributed table failover benchmark
* 50b29ce feat(api): expose batch query endpoint for reconciliation
`
}

// GenerateFakeDockerScreen returns a clean docker ps screen.
func GenerateFakeDockerScreen() string {
	return `CONTAINER ID   IMAGE                 COMMAND                  CREATED         STATUS         PORTS                    NAMES
8f921ab02c11   redis:7.2-alpine      "docker-entrypoint.s…"   2 days ago      Up 2 days      0.0.0.0:6379->6379/tcp   redis-cluster-master
b4109ca82d90   postgres:16-bullseye  "docker-entrypoint.s…"   5 days ago      Up 5 days      0.0.0.0:5432->5432/tcp   pg-main-db
3a2901ce48b2   local/gateway:v1.4    "/app/gateway-bin"       12 hours ago    Up 12 hours    0.0.0.0:8080->8080/tcp   edge-gateway-router
`
}

// ExtractMiniTicker extracts a continuous sliding window for status-line reading.
func ExtractMiniTicker(paragraphs []string, charOffset int, windowSize int) (visible string, percent int) {
	if len(paragraphs) == 0 {
		return "", 100
	}
	if windowSize <= 0 {
		windowSize = 40
	}

	// Join all paragraphs with separator
	joined := strings.Join(paragraphs, "  ")
	runes := []rune(joined)
	total := len(runes)
	if total == 0 {
		return "", 100
	}

	if charOffset < 0 {
		charOffset = 0
	}
	if charOffset >= total {
		charOffset = max(0, total-windowSize)
	}

	end := min(total, charOffset+windowSize)
	percent = int(float64(end) / float64(total) * 100)

	visible = string(runes[charOffset:end])
	return visible, percent
}
