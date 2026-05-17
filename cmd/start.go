package cmd

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/dan-slater/cc-lens/internal/server"
)

func Start(args []string) error {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	addr := fs.String("addr", envOr("CC_LENS_ADDR", ":8787"), "listen address")
	token := fs.String("token", os.Getenv("CC_LENS_TOKEN"), "bearer token (empty disables auth)")
	ringSize, _ := strconv.Atoi(envOr("CC_LENS_RING_SIZE", "1000"))
	ring := fs.Int("ring", ringSize, "event ring buffer size")
	webhook := fs.String("webhook", os.Getenv("CC_LENS_WEBHOOK_URL"), "webhook URL for event fan-out")
	kinds := fs.String("webhook-kinds", os.Getenv("CC_LENS_WEBHOOK_KINDS"), "comma-separated kinds to fan out (empty = all)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s := server.New(server.Config{
		Addr:         *addr,
		Token:        *token,
		RingSize:     *ring,
		WebhookURL:   *webhook,
		WebhookKinds: *kinds,
	})
	fmt.Printf("cc-lens listening on %s\n", *addr)
	return s.ListenAndServe()
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
