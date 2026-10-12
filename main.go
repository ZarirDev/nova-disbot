package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var (
	Token          string
	GuildID        string
	TargetURL      string
	AlertChannelID string
	CheckInterval  = 30 * time.Second
	FailureLimit   = 3
	RecoveryLimit  = 2
)

func init() {
	_ = godotenv.Load()

	Token = os.Getenv("DISCORD_BOT_TOKEN")
	GuildID = os.Getenv("DISCORD_GUILD_ID")
	TargetURL = getEnv("TARGET_URL", "https://home-bot.yourdomain.com")
	AlertChannelID = os.Getenv("DISCORD_CHANNEL_ID")
}

type ServerState string

const (
	StateOnline  ServerState = "ONLINE"
	StateOffline ServerState = "OFFLINE"
)

type Monitor struct {
	mu              sync.Mutex
	state           ServerState
	consecutiveFail int
	consecutiveSucc int
	lastLatency     time.Duration
	lastCheck       time.Time
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func main() {
	if Token == "" || AlertChannelID == "" {
		log.Fatal("Error: DISCORD_BOT_TOKEN and DISCORD_CHANNEL_ID environment variables must be set.")
	}

	// Initialize Discord Session
	sess, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

	monitor := &Monitor{
		state: StateOnline,
	}

	// Register Slash Commands Handler
	sess.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.ApplicationCommandData().Name == "status" {
			handleStatusCommand(s, i, monitor)
		} else if i.ApplicationCommandData().Name == "ping" {
			handlePingCommand(s, i)
		}
	})

	sess.Identify.Intents = discordgo.IntentsGuilds

	err = sess.Open()
	if err != nil {
		log.Fatalf("Error opening connection to Discord: %v", err)
	}
	defer sess.Close()

	registerCommands(sess)

	log.Printf("Bot started. Monitoring target URL: %s", TargetURL)

	// Start Background Monitoring Loop
	go monitor.startLoop(sess)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop
	log.Println("Shutting down nova-disbot...")
}

func (m *Monitor) startLoop(s *discordgo.Session) {
	ticker := time.NewTicker(CheckInterval)
	for range ticker.C {
		success, latency := checkURL(TargetURL)
		now := time.Now()

		m.mu.Lock()
		m.lastCheck = now
		m.lastLatency = latency

		if success {
			m.consecutiveSucc++
			m.consecutiveFail = 0

			if m.state == StateOffline && m.consecutiveSucc >= RecoveryLimit {
				m.state = StateOnline
				m.mu.Unlock()
				s.ChannelMessageSend(AlertChannelID, fmt.Sprintf("🟢 **Server Recovered!**\nTarget ` %s ` is back online.", TargetURL))
			} else {
				m.mu.Unlock()
			}
		} else {
			m.consecutiveFail++
			m.consecutiveSucc = 0

			if m.state == StateOnline && m.consecutiveFail >= FailureLimit {
				m.state = StateOffline
				m.mu.Unlock()
				s.ChannelMessageSend(AlertChannelID, fmt.Sprintf("🔴 **Server Went Offline!**\nCritical: Target ` %s ` failed %d consecutive checks.", TargetURL, FailureLimit))
			} else {
				m.mu.Unlock()
			}
		}
	}
}

func checkURL(url string) (bool, time.Duration) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	start := time.Now()
	resp, err := client.Get(url)
	latency := time.Since(start)

	if err != nil {
		return false, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, latency
	}

	return true, latency
}

func registerCommands(s *discordgo.Session) {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "status",
			Description: "Shows whether the home server is online or offline",
		},
		{
			Name:        "ping",
			Description: "Performs an immediate connectivity check",
		},
	}

	for _, cmd := range commands {
		s.ApplicationCommandCreate(s.State.User.ID, GuildID, cmd)
	}
}

func handleStatusCommand(s *discordgo.Session, i *discordgo.InteractionCreate, m *Monitor) {
	m.mu.Lock()
	state := m.state
	latency := m.lastLatency
	lastCheck := m.lastCheck
	m.mu.Unlock()

	statusEmoji := "🟢"
	statusText := "ONLINE"
	if state == StateOffline {
		statusEmoji = "🔴"
		statusText = "OFFLINE"
	}

	var lastCheckStr string
	if lastCheck.IsZero() {
		lastCheckStr = "Never"
	} else {
		lastCheckStr = lastCheck.Format("15:04:05 UTC")
	}

	content := fmt.Sprintf("%s **Status:** %s\n⏱️ **Response Time:** %v\n🕒 **Last Check:** %s",
		statusEmoji, statusText, latency, lastCheckStr)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content},
	})
}

func handlePingCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: "🔍 Running immediate connectivity check..."},
	})

	success, latency := checkURL(TargetURL)
	result := fmt.Sprintf("🏓 **Immediate Check Result:** Success: ` %v `, Latency: ` %v `", success, latency)

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &result,
	})
}
