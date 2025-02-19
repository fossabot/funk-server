package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/fasibio/funk-server/logger"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{} // use default options

type DataServiceWebSocket struct {
	ClientConnections map[string]*websocket.Conn
	genUID            func() (string, error)
	Db                ElsticConnection
	ConnectionAllowed func(*http.Request) bool
}

func (u *DataServiceWebSocket) Root(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hallo vom Server"))
}

func getIndexDate(time time.Time) string {
	return time.Format("2006-01-02")
}

func getLoggerWithSubscriptionID(logs *zap.SugaredLogger, uuid string) *zap.SugaredLogger {
	return logs.With(
		"subscriptionID", uuid,
	)
}

func (u *DataServiceWebSocket) interpretMessage(messages []Message, logs *zap.SugaredLogger) {
	for _, msg := range messages {
		str := msg.Data
		var d interface{}

		for _, v := range str {
			err := json.Unmarshal([]byte(v), &d)
			if err != nil {
				logs.Errorw("error by unmarshal data:" + err.Error())
				d = v
			}
			switch msg.Type {
			case MessageType_Log:
				u.Db.AddLog(LogData{
					Timestamp:  msg.Time,
					Type:       string(msg.Type),
					Logs:       d,
					Attributes: msg.Attributes,
				}, msg.SearchIndex+"_funk-"+getIndexDate(time.Now()))

			case MessageType_Stats:
				{
					u.Db.AddStats(StatsData{
						Timestamp:  msg.Time,
						Type:       string(msg.Type),
						Stats:      d,
						Attributes: msg.Attributes,
					}, msg.SearchIndex+"_funk-"+getIndexDate(time.Now()))
				}
			}
		}
	}
}

func (u *DataServiceWebSocket) messageSubscribeHandler(uuid string, c *websocket.Conn) {
	logs := getLoggerWithSubscriptionID(logger.Get(), uuid)
	for {
		var messages []Message
		err := c.ReadJSON(&messages)
		if err != nil {
			logs.Errorw("error by ClientConn" + err.Error())
			delete(u.ClientConnections, uuid)
			return
		}

		u.interpretMessage(messages, logs)
	}
}

func (u *DataServiceWebSocket) Subscribe(w http.ResponseWriter, r *http.Request) {
	if !u.ConnectionAllowed(r) {
		logger.Get().Infow("Connection forbidden to subscribe")
		w.WriteHeader(401)
		return
	}
	uuid, _ := u.genUID()
	logs := getLoggerWithSubscriptionID(logger.Get(), uuid)
	logs.Infow("New Subscribe Client")
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logs.Errorw("error by Subscribe" + err.Error())
		return
	}

	go u.messageSubscribeHandler(uuid, c)

	u.ClientConnections[uuid] = c
}
