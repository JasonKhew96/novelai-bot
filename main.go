package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"encoding/base64"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/styling"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
)

type QueueItem struct {
	StartTime   time.Time
	Entities    tg.Entities
	Updates     *tg.UpdateNewChannelMessage
	IsNSFW      bool
	IsLandscape bool
	Input       string
}

func main() {
	log.Println("Starting...")
	config, err := loadConfig()
	if err != nil {
		panic(err)
	}
	// log.Println(config)

	// logger, _ := zap.NewDevelopment()

	var queues chan *QueueItem
	queues = make(chan *QueueItem, 100)

	dispatcher := tg.NewUpdateDispatcher()
	client := telegram.NewClient(config.ApiId, config.ApiHash, telegram.Options{
		// Logger:        logger,
		UpdateHandler: dispatcher,
	})
	if err := client.Run(context.Background(), func(ctx context.Context) error {
		api := client.API()

		auth, err := client.Auth().Bot(ctx, config.BotToken)
		if err != nil {
			return err
		}

		log.Println(auth.User.GetID())

		up := uploader.NewUploader(api)
		sender := message.NewSender(api).WithUploader(up)

		go processQueue(config, up, sender, queues)

		dispatcher.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
			m, ok := u.Message.(*tg.Message)
			if !ok || m.Out {
				return nil
			}

			switch v := m.PeerID.(type) {
			case *tg.PeerChannel:
				if v.ChannelID != config.WhitelistChatId {
					return nil
				}
			default:
				return nil
			}

			if len(m.Entities) <= 0 {
				return nil
			}

			switch v := m.Entities[0].(type) {
			case *tg.MessageEntityBotCommand:
				if v.Offset != 0 {
					return nil
				}
			default:
				return nil
			}

			var isNSFW, isLandscape bool

			switch {
			case strings.HasPrefix(m.Message, "/gen_s_p"):
				isNSFW = false
				isLandscape = false
			case strings.HasPrefix(m.Message, "/gen_s_l"):
				isNSFW = false
				isLandscape = true
			case strings.HasPrefix(m.Message, "/gen_n_p"):
				isNSFW = true
				isLandscape = false
			case strings.HasPrefix(m.Message, "/gen_n_l"):
				isNSFW = true
				isLandscape = true
			default:
				return nil
			}

			splits := strings.Split(m.Message, " ")
			if len(splits) < 2 {
				return nil
			}
			input := strings.Join(splits[1:], " ")

			queues <- &QueueItem{
				StartTime:   time.Now(),
				Entities:    e,
				Updates:     u,
				IsNSFW:      isNSFW,
				IsLandscape: isLandscape,
				Input:       input,
			}
			queueLen := len(queues)

			_, err = sender.Reply(e, u).Textf(context.Background(), "列队中...\n长度: %d", queueLen)
			return err
		})

		<-ctx.Done()
		return ctx.Err()
	}); err != nil {
		panic(err)
	}
}

func processQueue(config *Config, up *uploader.Uploader, sender *message.Sender, queues <-chan *QueueItem) {
	for queue := range queues {
		queueSeconds := time.Now().Sub(queue.StartTime).Seconds()
		reqStartTime := time.Now()

		resp, err := genImage(config.BearerToken, queue.Input, queue.IsNSFW, queue.IsLandscape)
		if err != nil {
			log.Println("genImage: ", err)
			sender.Reply(queue.Entities, queue.Updates).Text(context.Background(), "生成失败")
			continue
		}

		data, err := base64.StdEncoding.DecodeString(string(resp))
		if err != nil {
			log.Println(err)
			sender.Reply(queue.Entities, queue.Updates).Text(context.Background(), "生成失败")
			continue
		}

		timeNowUnix := time.Now().Unix()

		fn := fmt.Sprintf("%d.png", timeNowUnix)
		upload, err := up.FromBytes(context.Background(), fn, data)
		if err != nil {
			log.Println(err)
			sender.Reply(queue.Entities, queue.Updates).Text(context.Background(), "生成失败")
			continue
		}

		reqSeconds := time.Now().Sub(reqStartTime).Seconds()

		photo := message.UploadedPhoto(upload, styling.Plain(fmt.Sprintf("列队耗时 %.2f 秒\n生成耗时 %.2f 秒", queueSeconds, reqSeconds)))

		doc := message.UploadedDocument(upload)
		doc.ForceFile(true).Filename(fn).MIME("image/png")

		// Sending reply.
		if _, err = sender.Reply(queue.Entities, queue.Updates).Media(context.Background(), photo); err != nil {
			log.Println(err)
			continue
		}

		if _, err = sender.Reply(queue.Entities, queue.Updates).Media(context.Background(), doc); err != nil {
			log.Println(err)
			continue
		}

		time.Sleep(time.Second * 5)
	}
}
