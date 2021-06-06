package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
)

func TestE2E(t *testing.T) {
	testEngine(t, func(s *Server, storage updates.Storage) chan *tg.Updates {
		c := make(chan *tg.Updates, 10)

		var (
			biba = s.peers.createUser("biba")
			boba = s.peers.createUser("boba")
			chat = s.peers.createChat("chat")
		)

		var channels []*tg.PeerChannel
		for i := 0; i < 10; i++ {
			c := s.peers.createChannel(fmt.Sprintf("channel-%d", i))
			require.NoError(t, storage.SetChannelPts(c.ChannelID, 0))
			channels = append(channels, c)
		}

		require.NoError(t, storage.SetState(updates.State{}))

		var wg sync.WaitGroup
		wg.Add(2)

		// Biba.
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				c <- s.CreateEvent(func(ev *EventBuilder) {
					ev.SendMessage(biba, chat, fmt.Sprintf("biba-%d", i))

					for _, c := range channels {
						ev.SendMessage(biba, c, fmt.Sprintf("biba-channel-%d", i))
					}
				})
			}
		}()

		// Boba.
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				c <- s.CreateEvent(func(ev *EventBuilder) {
					ev.SendMessage(boba, chat, fmt.Sprintf("boba-%d", i))

					for _, c := range channels {
						ev.SendMessage(boba, c, fmt.Sprintf("boba-channel-%d", i))
					}
				})
			}
		}()

		go func() {
			wg.Wait()
			close(c)
		}()
		return c
	})
}

func testEngine(t *testing.T, f func(s *Server, storage updates.Storage) chan *tg.Updates) {
	t.Helper()

	var (
		log     = zaptest.NewLogger(t)
		s       = NewServer()
		h       = NewHandler()
		storage = updates.NewMemStorage()
	)

	uchan := f(s, storage)
	e := updates.New(updates.Config{
		RawClient: s,
		Handler:   h,
		SelfID:    123,
		Storage:   storage,
		Logger:    log.Named("gaps"),
	})

	err := e.Run(context.Background(), func(ctx context.Context) error {
		for u := range reorder(loss(uchan)) {
			if err := e.HandleUpdates(u); err != nil {
				return err
			}
		}

		var updates []tg.UpdateClass
		updates = append(updates, &tg.UpdatePtsChanged{})
		if err := storage.Channels(func(channelID, pts int) {
			updates = append(updates, &tg.UpdateChannelTooLong{
				ChannelID: channelID,
			})
		}); err != nil {
			return err
		}

		if err := e.HandleUpdates(&tg.Updates{
			Updates: updates,
		}); err != nil {
			return err
		}

		return nil
	})
	require.NoError(t, err)

	require.Equal(t, s.messages, h.messages)
	require.Equal(t, s.peers.channels, h.ents.Channels)
	require.Equal(t, s.peers.chats, h.ents.Chats)
	require.Equal(t, s.peers.users, h.ents.Users)
}

func reorder(in chan *tg.Updates) chan *tg.Updates {
	out := make(chan *tg.Updates)
	var buf []*tg.Updates

	go func() {
		defer close(out)

		for u := range in {
			if rand.Intn(2) == 1 {
				buf = append(buf, u)
				continue
			}

			out <- u

			for _, u := range buf {
				out <- u
			}
		}

		for _, u := range buf {
			out <- u
		}
	}()

	return out
}

func loss(in chan *tg.Updates) chan *tg.Updates {
	out := make(chan *tg.Updates)

	go func() {
		defer close(out)

		for u := range in {
			if rand.Intn(2) == 1 {
				continue
			}

			out <- u
		}
	}()

	return out
}
