package main

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/LittleXiaYuan/ledger"

	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/execution/channel"
	iledger "yunque-agent/internal/ledger"
)

// initChannels initializes all communication channels (Telegram, Feishu, Discord,
// WhatsApp, Signal, Slack, QQ, Email, WeCom, DingTalk, WeChat, LINE, Kook, Satori).
// Extracted from main.go lines 624-794.
func initChannels(app *agentrt.App) error {
	cfg := app.Config
	channelReg := channel.NewRegistry()
	channelReg.SetMetricsHooks(app.Metrics.RecordChannelMessage, app.Metrics.RecordChannelSend)
	groupTracker := channel.NewGroupTracker(cfg.DataPath("groups.json"))
	channelReg.SetGroupTracker(groupTracker)
	channelReg.SetGroupFilter(channel.LoadGroupFilterConfig())

	// Wire GroupTracker → Ledger KV
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			migrator := iledger.NewKVMigrator(ldg)
			_ = migrator.MigrateFile("channel", "groups", cfg.DataPath("groups.json"))
			groupTracker.SetKVStore(iledger.NewKVConfigStore(ldg, "channel"))
			slog.Info("group tracker wired to Ledger KV")
		}
	}
	engagementProfile := channel.LoadEngagementMode()
	channelReg.SetEngagement(engagementProfile)
	app.Set("engagement_profile", engagementProfile)

	// ── Channel Registration ──

	if tgToken := os.Getenv("TELEGRAM_BOT_TOKEN"); tgToken != "" {
		channelReg.Register(channel.NewTelegram(tgToken))
		slog.Info("telegram channel registered")
	}

	if fsID := os.Getenv("FEISHU_APP_ID"); fsID != "" {
		fsSecret := os.Getenv("FEISHU_APP_SECRET")
		fsEncryptKey := os.Getenv("FEISHU_ENCRYPT_KEY")
		channelReg.Register(channel.NewFeishu(fsID, fsSecret, fsEncryptKey))
		slog.Info("feishu channel registered")
	}

	if discordToken := os.Getenv("DISCORD_BOT_TOKEN"); discordToken != "" {
		channelReg.Register(channel.NewDiscord(discordToken))
		slog.Info("discord channel registered")
	}

	if waPhoneID := os.Getenv("WHATSAPP_PHONE_NUMBER_ID"); waPhoneID != "" {
		channelReg.Register(channel.NewWhatsApp(channel.WhatsAppConfig{
			PhoneNumberID: waPhoneID,
			AccessToken:   os.Getenv("WHATSAPP_ACCESS_TOKEN"),
			VerifyToken:   os.Getenv("WHATSAPP_VERIFY_TOKEN"),
			WebhookPath:   os.Getenv("WHATSAPP_WEBHOOK_PATH"),
		}))
		slog.Info("whatsapp channel registered")
	}

	if signalNumber := os.Getenv("SIGNAL_PHONE_NUMBER"); signalNumber != "" {
		channelReg.Register(channel.NewSignal(channel.SignalConfig{
			PhoneNumber: signalNumber,
			ConfigDir:   os.Getenv("SIGNAL_CONFIG_DIR"),
		}))
		slog.Info("signal channel registered")
	}

	if slackBotToken := os.Getenv("SLACK_BOT_TOKEN"); slackBotToken != "" {
		channelReg.Register(channel.NewSlack(channel.SlackConfig{
			BotToken:      slackBotToken,
			AppToken:      os.Getenv("SLACK_APP_TOKEN"),
			SigningSecret: os.Getenv("SLACK_SIGNING_SECRET"),
			WebhookPath:   os.Getenv("SLACK_WEBHOOK_PATH"),
		}))
		slog.Info("slack channel registered")
	}

	if qqAppID := os.Getenv("QQ_APP_ID"); qqAppID != "" {
		channelReg.Register(channel.NewQQ(channel.QQConfig{
			AppID:     qqAppID,
			AppSecret: os.Getenv("QQ_APP_SECRET"),
			Sandbox:   os.Getenv("QQ_SANDBOX") == "true",
		}))
		slog.Info("qq channel registered")
	}

	if smtpHost := os.Getenv("SMTP_HOST"); smtpHost != "" {
		smtpPort := 587
		if p := os.Getenv("SMTP_PORT"); p != "" {
			if n, err := strconv.Atoi(p); err == nil && n > 0 {
				smtpPort = n
			}
		}
		channelReg.Register(channel.NewEmail(channel.EmailConfig{
			Host:     smtpHost,
			Port:     smtpPort,
			Username: os.Getenv("SMTP_USERNAME"),
			Password: os.Getenv("SMTP_PASSWORD"),
			From:     os.Getenv("SMTP_FROM"),
			UseTLS:   os.Getenv("SMTP_USE_TLS") == "true",
		}))
		slog.Info("email channel registered", "host", smtpHost, "port", smtpPort)
	}

	if wecomCorpID := os.Getenv("WECOM_CORPID"); wecomCorpID != "" {
		channelReg.Register(channel.NewWeCom(channel.WeComConfig{
			CorpID:     wecomCorpID,
			AgentID:    os.Getenv("WECOM_AGENT_ID"),
			Secret:     os.Getenv("WECOM_SECRET"),
			Token:      os.Getenv("WECOM_TOKEN"),
			AESKey:     os.Getenv("WECOM_AES_KEY"),
			Port:       os.Getenv("WECOM_PORT"),
			BindAddr:   os.Getenv("WECOM_BIND_ADDR"),
			APIBaseURL: os.Getenv("WECOM_API_BASE_URL"),
		}))
		slog.Info("wecom channel registered", "corpid", wecomCorpID)
	}

	if dtClientID := os.Getenv("DINGTALK_CLIENT_ID"); dtClientID != "" {
		channelReg.Register(channel.NewDingTalk(channel.DingTalkConfig{
			ClientID:     dtClientID,
			ClientSecret: os.Getenv("DINGTALK_CLIENT_SECRET"),
			RobotCode:    os.Getenv("DINGTALK_ROBOT_CODE"),
			Port:         os.Getenv("DINGTALK_PORT"),
			BindAddr:     os.Getenv("DINGTALK_BIND_ADDR"),
		}))
		slog.Info("dingtalk channel registered", "client_id", dtClientID)
	}

	if wxAppID := os.Getenv("WECHAT_APPID"); wxAppID != "" {
		channelReg.Register(channel.NewWeChatOfficial(channel.WeChatOfficialConfig{
			AppID:          wxAppID,
			AppSecret:      os.Getenv("WECHAT_APPSECRET"),
			Token:          os.Getenv("WECHAT_TOKEN"),
			EncodingAESKey: os.Getenv("WECHAT_AES_KEY"),
			Port:           os.Getenv("WECHAT_PORT"),
			BindAddr:       os.Getenv("WECHAT_BIND_ADDR"),
			APIBaseURL:     os.Getenv("WECHAT_API_BASE_URL"),
		}))
		slog.Info("wechat official channel registered", "appid", wxAppID)
	}

	if lineSecret := os.Getenv("LINE_CHANNEL_SECRET"); lineSecret != "" {
		channelReg.Register(channel.NewLINE(channel.LINEConfig{
			ChannelSecret: lineSecret,
			ChannelToken:  os.Getenv("LINE_CHANNEL_TOKEN"),
			Port:          os.Getenv("LINE_PORT"),
			BindAddr:      os.Getenv("LINE_BIND_ADDR"),
		}))
		slog.Info("line channel registered")
	}

	if kookToken := os.Getenv("KOOK_TOKEN"); kookToken != "" {
		channelReg.Register(channel.NewKook(channel.KookConfig{
			Token: kookToken,
		}))
		slog.Info("kook channel registered")
	}

	if satoriEndpoint := os.Getenv("SATORI_ENDPOINT"); satoriEndpoint != "" {
		channelReg.Register(channel.NewSatori(channel.SatoriConfig{
			Endpoint: satoriEndpoint,
			Token:    os.Getenv("SATORI_TOKEN"),
			Port:     os.Getenv("SATORI_PORT"),
			BindAddr: os.Getenv("SATORI_BIND_ADDR"),
			Platform: os.Getenv("SATORI_PLATFORM"),
			SelfID:   os.Getenv("SATORI_SELF_ID"),
		}))
		slog.Info("satori channel registered", "endpoint", satoriEndpoint)
	}

	app.Set(agentrt.CompChannelReg, channelReg)
	slog.Info("channel registry initialized", "channels", len(channelReg.All()))

	return nil
}
