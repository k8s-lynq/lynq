package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/k8s-lynq/lynq/dashboard/bff/internal/api"
	"github.com/k8s-lynq/lynq/dashboard/bff/internal/kube"
)

func main() {
	var (
		addr        = flag.String("addr", ":8080", "HTTP server address")
		appMode     = flag.String("mode", "local", "Application mode: local or cluster")
		kubeconfig  = flag.String("kubeconfig", "", "Path to kubeconfig file (local mode only)")
		kubecontext = flag.String("context", "", "Kubernetes context to use (local mode only)")
	)
	flag.Parse()

	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Lynq Dashboard BFF", "addr", *addr, "mode", *appMode)

	// Initialize Kubernetes client
	kubeClient, err := kube.NewClient(*appMode, *kubeconfig, *kubecontext)
	if err != nil {
		slog.Error("Failed to create Kubernetes client", "error", err)
		os.Exit(1)
	}

	// Handle context selection for local mode
	if *appMode == "local" {
		currentContext := kubeClient.GetCurrentContext()

		// If no context specified and multiple contexts available, prompt user
		if *kubecontext == "" && currentContext == "" {
			contexts, err := kubeClient.ListContexts()
			if err == nil && len(contexts) > 0 {
				selectedContext := promptContextSelection(contexts)
				if selectedContext != "" {
					if err := kubeClient.SwitchContext(selectedContext); err != nil {
						slog.Error("Failed to switch context", "error", err)
						os.Exit(1)
					}
					currentContext = selectedContext
				}
			}
		}

		if currentContext == "" {
			slog.Warn("No Kubernetes context selected. Use /api/v1/contexts to list and switch contexts.")
		} else {
			slog.Info("Using Kubernetes context", "context", currentContext)
		}
	}

	// Create router
	router := api.NewRouter(kubeClient, *appMode)

	// Create server
	server := &http.Server{
		Addr:         *addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("Server listening", "addr", *addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}

	slog.Info("Server stopped")
}

// promptContextSelection shows available contexts and prompts user to select one
func promptContextSelection(contexts []kube.KubeContext) string {
	if len(contexts) == 0 {
		return ""
	}

	fmt.Println("\nAvailable Kubernetes contexts:")
	fmt.Println("─────────────────────────────────")
	for i, ctx := range contexts {
		marker := "  "
		if ctx.Current {
			marker = "* "
		}
		ns := ctx.Namespace
		if ns == "" {
			ns = "default"
		}
		fmt.Printf("%s[%d] %s (cluster: %s, namespace: %s)\n", marker, i+1, ctx.Name, ctx.Cluster, ns)
	}
	fmt.Println("─────────────────────────────────")
	fmt.Print("Select context [1-", len(contexts), "]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(contexts) {
		fmt.Println("Invalid selection")
		return ""
	}

	return contexts[idx-1].Name
}
