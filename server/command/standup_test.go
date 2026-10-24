package command

import (
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/standup-raven/standup-raven/server/otime"
	"github.com/standup-raven/standup-raven/server/standup/notification"
)

func reportCommandContext(dates ...otime.OTime) Context {
	return Context{
		CommandArgs: &model.CommandArgs{ChannelId: testChannelID, UserId: testUserID},
		Props:       map[string]interface{}{"visibility": "public", "dates": dates},
	}
}

func reportDate(day int) otime.OTime {
	return otime.OTime{Time: time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC)}
}

func TestExecuteCommandStandup_Generated(t *testing.T) {
	defer monkey.UnpatchAll()
	mockCommandAPI()

	var sent []string
	monkey.Patch(notification.SendStandupReport, func(channelIDs []string, date otime.OTime, visibility, userID string, updateStatus bool) error {
		sent = append(sent, date.Format(dateLayout)+" "+visibility+" "+channelIDs[0])
		return nil
	})

	response, appErr := executeCommandStandup(nil, reportCommandContext(reportDate(23), reportDate(24)))
	assert.Nil(t, appErr)
	assert.Equal(t, "Standup reports generated", response.Text)
	assert.Equal(t, []string{"23-10-2026 public channel_id", "24-10-2026 public channel_id"}, sent)
}

func TestExecuteCommandStandup_ReportsFailures(t *testing.T) {
	defer monkey.UnpatchAll()
	mockCommandAPI()

	monkey.Patch(notification.SendStandupReport, func(channelIDs []string, date otime.OTime, visibility, userID string, updateStatus bool) error {
		if date.Day() == 24 {
			return errors.New("couldn't create the post")
		}
		return nil
	})

	response, appErr := executeCommandStandup(nil, reportCommandContext(reportDate(23), reportDate(24)))
	assert.Nil(t, appErr)
	assert.Equal(t, model.CommandResponseTypeEphemeral, response.ResponseType)
	assert.Equal(t, "Couldn't generate the standup report for:\n* 24-10-2026: couldn't create the post", response.Text,
		"the user is told which date failed and why, rather than that the reports were generated")
}
