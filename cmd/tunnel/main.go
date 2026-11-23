package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/IbodovElbek/tunnel-client/internal/build"
	"github.com/IbodovElbek/tunnel-client/internal/client"
	"github.com/IbodovElbek/tunnel-client/internal/proto"
	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{Use: "tunnel", Short: "Tunnel klienti"}
	cmd.Version = build.String()
	cmd.SetVersionTemplate("tunnel {{.Version}}\n")
	var subdomain, customDomain, serverAddr, tokenValue string
	var insecure, httpPlain bool
	httpCmd := &cobra.Command{Use: "http LOCAL", Short: "HTTP tunnel ochadi", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, a []string) error {
		if tokenValue == "" {
			tokenValue = os.Getenv("TUNNEL_TOKEN")
		}
		creds, err := client.LoadCredentials()
		if err != nil && tokenValue == "" {
			return err
		}
		if serverAddr != "" {
			creds.Server = serverAddr
		}
		if tokenValue != "" {
			creds.Token = tokenValue
		}
		if creds.Server == "" || creds.Token == "" {
			return fmt.Errorf("--server va --token (yoki TUNNEL_TOKEN/tunnel login) kerak")
		}
		if insecure {
			return fmt.Errorf("--insecure o'rniga lokal sinov uchun --plain ishlating")
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return (&client.Client{Config: client.Config{Server: creds.Server, Token: creds.Token, Version: build.Version, Plain: httpPlain}}).RunTunnel(ctx, client.TunnelRequest{ID: subdomain, Kind: "http", Local: a[0], Subdomain: subdomain, Domain: customDomain}, client.NewAgent())
	}}
	httpCmd.Flags().StringVar(&subdomain, "subdomain", "", "Subdomen")
	httpCmd.Flags().StringVar(&customDomain, "domain", "", "Tasdiqlangan custom domen")
	httpCmd.Flags().StringVar(&serverAddr, "server", "", "Control server manzili")
	httpCmd.Flags().StringVar(&tokenValue, "token", "", "Kirish tokeni")
	httpCmd.Flags().BoolVar(&insecure, "insecure", false, "Eskirgan: --plain dan foydalaning")
	httpCmd.Flags().BoolVar(&httpPlain, "plain", false, "TLSsiz, faqat 127.0.0.1/::1 uchun")
	cmd.AddCommand(httpCmd)
	var remotePort int
	var tcpPlain bool
	tcpCmd := &cobra.Command{Use: "tcp LOCAL", Short: "TCP tunnel ochadi", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, a []string) error {
		creds, err := client.LoadCredentials()
		if err != nil && tokenValue == "" {
			return err
		}
		if serverAddr != "" {
			creds.Server = serverAddr
		}
		if tokenValue != "" {
			creds.Token = tokenValue
		}
		if creds.Server == "" || creds.Token == "" {
			return fmt.Errorf("--server va --token (yoki tunnel login) kerak")
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return (&client.Client{Config: client.Config{Server: creds.Server, Token: creds.Token, Version: build.Version, Plain: tcpPlain}}).RunTunnel(ctx, client.TunnelRequest{ID: fmt.Sprintf("tcp-%d", remotePort), Kind: "tcp", Local: a[0], RemotePort: remotePort}, client.NewAgent())
	}}
	tcpCmd.Flags().StringVar(&serverAddr, "server", "", "Control server manzili")
	tcpCmd.Flags().StringVar(&tokenValue, "token", "", "Kirish tokeni")
	tcpCmd.Flags().IntVar(&remotePort, "remote-port", 0, "Public TCP port (0 avtomatik ajratadi)")
	tcpCmd.Flags().BoolVar(&tcpPlain, "plain", false, "TLSsiz, faqat 127.0.0.1/::1 uchun")
	cmd.AddCommand(tcpCmd)
	var tlsDomain, tlsServer, tlsToken string
	var tlsPlain bool
	tlsCmd := &cobra.Command{Use: "tls LOCAL", Short: "TLS-SNI passthrough tunnel ochadi", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, a []string) error {
		creds, err := client.LoadCredentials()
		if err != nil && tlsToken == "" {
			return err
		}
		if tlsServer != "" {
			creds.Server = tlsServer
		}
		if tlsToken != "" {
			creds.Token = tlsToken
		}
		if creds.Server == "" || creds.Token == "" || tlsDomain == "" {
			return fmt.Errorf("--server, --token va --domain kerak")
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return (&client.Client{Config: client.Config{Server: creds.Server, Token: creds.Token, Version: build.Version, Plain: tlsPlain}}).RunTunnel(ctx, client.TunnelRequest{ID: tlsDomain, Kind: "tls", Local: a[0], Domain: tlsDomain}, client.NewAgent())
	}}
	tlsCmd.Flags().StringVar(&tlsDomain, "domain", "", "Tasdiqlangan TLS domen")
	tlsCmd.Flags().StringVar(&tlsServer, "server", "", "Control server manzili")
	tlsCmd.Flags().StringVar(&tlsToken, "token", "", "Kirish tokeni")
	tlsCmd.Flags().BoolVar(&tlsPlain, "plain", false, "TLSsiz, faqat 127.0.0.1/::1 uchun")
	cmd.AddCommand(tlsCmd)
	var loginServer string
	login := &cobra.Command{Use: "login", Short: "Tokenni xavfsiz saqlaydi", RunE: func(cmd *cobra.Command, _ []string) error {
		token, _ := cmd.Flags().GetString("token")
		if token == "" {
			return fmt.Errorf("--token berilishi kerak")
		}
		if loginServer == "" {
			loginServer = "tunel.ibodov.uz:7000"
		}
		if err := client.SaveCredentials(client.Credentials{Server: loginServer, Token: token}); err != nil {
			return err
		}
		fmt.Println("Token saqlandi.")
		return nil
	}}
	login.Flags().String("token", "", "Kirish tokeni")
	login.Flags().StringVar(&loginServer, "server", "tunel.ibodov.uz:7000", "Control server manzili")
	cmd.AddCommand(login)

	var domainServer, domainToken string
	var domainPlain bool
	domainCmd := &cobra.Command{Use: "domain", Short: "Custom domenlarni boshqaradi"}
	runDomain := func(kind string, args []string) error {
		creds, err := client.LoadCredentials()
		if err != nil && domainToken == "" {
			return err
		}
		if domainServer != "" {
			creds.Server = domainServer
		}
		if domainToken != "" {
			creds.Token = domainToken
		}
		if creds.Server == "" || creds.Token == "" {
			return fmt.Errorf("--server va --token (yoki tunnel login) kerak")
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		request := proto.Message{Type: kind}
		if len(args) != 0 {
			request.Domain = args[0]
		}
		response, err := (&client.Client{Config: client.Config{Server: creds.Server, Token: creds.Token, Version: build.Version, Plain: domainPlain}}).RunCommand(ctx, request)
		if err != nil {
			return err
		}
		switch response.Type {
		case "domain_ok":
			fmt.Printf("Domen: %s\n", response.Domain)
			if response.VerificationToken != "" {
				fmt.Printf("TXT: _tunnel-verify.%s = %s\n", response.Domain, response.VerificationToken)
			}
			if response.Verified {
				fmt.Println("Tasdiqlandi.")
			}
		case "domain_removed":
			fmt.Printf("O'chirildi: %s\n", response.Domain)
		case "domain_list":
			for _, d := range response.Domains {
				fmt.Printf("%s\t%t\n", d.Name, d.Verified)
			}
		}
		return nil
	}
	domainCmd.AddCommand(
		&cobra.Command{Use: "add DOMAIN", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, a []string) error { return runDomain("domain_add", a) }},
		&cobra.Command{Use: "verify DOMAIN", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, a []string) error { return runDomain("domain_verify", a) }},
		&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, a []string) error { return runDomain("domain_list", a) }},
		&cobra.Command{Use: "remove DOMAIN", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, a []string) error { return runDomain("domain_remove", a) }},
	)
	domainCmd.PersistentFlags().StringVar(&domainServer, "server", "", "Control server manzili")
	domainCmd.PersistentFlags().StringVar(&domainToken, "token", "", "Kirish tokeni")
	domainCmd.PersistentFlags().BoolVar(&domainPlain, "plain", false, "TLSsiz, faqat 127.0.0.1/::1 uchun")
	cmd.AddCommand(domainCmd)

	var startConfig string
	var startPlain bool
	startCmd := &cobra.Command{Use: "start [NAME...]", Short: "tunnel.yml dagi tunnellarni ishga tushiradi", RunE: func(_ *cobra.Command, names []string) error {
		file, err := client.LoadTunnelFile(startConfig)
		if err != nil {
			return err
		}
		selected := make(map[string]bool)
		for _, name := range names {
			selected[name] = true
		}
		if len(selected) != 0 {
			for name := range selected {
				if _, ok := file.Tunnels[name]; !ok {
					return fmt.Errorf("tunnel topilmadi: %s", name)
				}
			}
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		var wg sync.WaitGroup
		errs := make(chan error, len(file.Tunnels))
		for name, spec := range file.Tunnels {
			if len(selected) != 0 && !selected[name] {
				continue
			}
			name, spec := name, spec
			wg.Add(1)
			go func() {
				defer wg.Done()
				request := client.TunnelRequest{ID: name, Kind: spec.Kind, Local: spec.Local, Subdomain: spec.Subdomain, Domain: spec.Domain, RemotePort: spec.RemotePort}
				if err := (&client.Client{Config: client.Config{Server: file.Server, Token: file.Token, Version: build.Version, Plain: startPlain}}).RunTunnel(ctx, request, client.NewAgent()); err != nil && ctx.Err() == nil {
					errs <- err
				}
			}()
		}
		finished := make(chan struct{})
		go func() { wg.Wait(); close(finished) }()
		select {
		case err := <-errs:
			return err
		case <-ctx.Done():
			<-finished
			return nil
		}
	}}
	startCmd.Flags().StringVar(&startConfig, "config", "tunnel.yml", "Tunnel konfiguratsiyasi")
	startCmd.Flags().BoolVar(&startPlain, "plain", false, "TLSsiz, faqat 127.0.0.1/::1 uchun")
	cmd.AddCommand(startCmd)
	cmd.AddCommand(&cobra.Command{Use: "status", Short: "Lokal client holatini ko'rsatadi", RunE: func(_ *cobra.Command, _ []string) error {
		creds, err := client.LoadCredentials()
		if err != nil {
			return err
		}
		fmt.Printf("Server: %s\nInspector: http://127.0.0.1:4040/\n", creds.Server)
		return nil
	}})
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Xato:", err)
		os.Exit(1)
	}
}
