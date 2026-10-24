package notification

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"

	"github.com/standup-raven/standup-raven/server/config"
	"github.com/standup-raven/standup-raven/server/otime"
	"github.com/standup-raven/standup-raven/server/standup"
)

func reportStylesConfig() *standup.Config {
	return &standup.Config{
		ChannelID: "channel_id",
		Sections:  []string{"Today", "Blockers"},
		Members:   []string{"user_id_1", "user_id_2"},
		Timezone:  "Asia/Kolkata",
	}
}

func reportStylesStandups() []*standup.UserStandup {
	return []*standup.UserStandup{
		{
			UserID:    "user_id_1",
			ChannelID: "channel_id",
			Standup: map[string]*[]string{
				"Today":    {"Internal sync with @maria"},
				"Blockers": {},
			},
		},
		{
			UserID:    "user_id_2",
			ChannelID: "channel_id",
			Standup: map[string]*[]string{
				"Today":    {"Release notes"},
				"Blockers": {"Too much mail"},
			},
		},
	}
}

func reportStylesSetUp(siteURL string) {
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("GetUser", "user_id_1").Return(&model.User{FirstName: "Foo", LastName: "Bar"}, nil)
	mockAPI.On("GetUser", "user_id_2").Return(&model.User{FirstName: "John", LastName: "Doe"}, nil)

	mmConfig := &model.Config{}
	if siteURL != "" {
		mmConfig.ServiceSettings.SiteURL = model.NewPointer(siteURL)
	}
	mockAPI.On("GetConfig").Return(mmConfig)
}

var reportStylesDate = otime.OTime{Time: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)}

func TestGenerateUserCompactStandupReport(t *testing.T) {
	defer TearDown()
	reportStylesSetUp("")

	report, err := generateUserCompactStandupReport(reportStylesConfig(), reportStylesStandups(), []string{"carol"}, nil, "channel_id", reportStylesDate)
	assert.NoError(t, err)

	assert.Equal(t, "#### Standup Report for *24 Oct 2026*\n"+
		"\n@carol has not submitted their standup.\n"+
		"\n---\n#### Foo Bar\n**Today**\n1. Internal sync with @maria\n"+
		"\n---\n#### John Doe\n**Today**\n1. Release notes\n\n**Blockers**\n1. Too much mail\n",
		report.Message)
	assert.NotContains(t, report.Message, "![", "the compact report has no pictures")
	assert.Empty(t, report.Type)
}

func TestGenerateUserCompactStandupReport_NoStandups(t *testing.T) {
	defer TearDown()
	reportStylesSetUp("")

	report, err := generateUserCompactStandupReport(reportStylesConfig(), nil, []string{"carol"}, nil, "channel_id", reportStylesDate)
	assert.NoError(t, err)
	assert.Equal(t, "#### Standup Report for *24 Oct 2026*\n\n:warning: **No user has submitted their standup.**", report.Message)
}

func TestGenerateUserAttachmentsStandupReport(t *testing.T) {
	defer TearDown()
	reportStylesSetUp("https://chat.example.com/")

	report, err := generateUserAttachmentsStandupReport(reportStylesConfig(), reportStylesStandups(), nil, []string{"dave"}, "channel_id", reportStylesDate)
	assert.NoError(t, err)

	assert.Equal(t, "#### Standup Report for *24 Oct 2026*\n\n@dave is out of office.", report.Message)

	attachments := report.Attachments()
	if !assert.Len(t, attachments, 2, "one attachment per member") {
		return
	}

	assert.Equal(t, "Foo Bar", attachments[0].AuthorName)
	assert.Equal(t, "https://chat.example.com/api/v4/users/user_id_1/image", attachments[0].AuthorIcon,
		"the picture is addressed absolutely so the mobile apps can load it")
	assert.Equal(t, "**Today**\n1. Internal sync with @maria", attachments[0].Text,
		"a section left empty is not shown")
	assert.Equal(t, "John Doe", attachments[1].AuthorName)
	assert.Equal(t, "**Today**\n1. Release notes\n\n**Blockers**\n1. Too much mail", attachments[1].Text)
	assert.NotEmpty(t, attachments[1].Fallback)
}

func TestGenerateUserAttachmentsStandupReport_NoSiteURL(t *testing.T) {
	defer TearDown()
	reportStylesSetUp("")

	report, err := generateUserAttachmentsStandupReport(reportStylesConfig(), reportStylesStandups(), nil, nil, "channel_id", reportStylesDate)
	assert.NoError(t, err)
	assert.Equal(t, "/api/v4/users/user_id_1/image", report.Attachments()[0].AuthorIcon)
}

func TestGenerateUserCardsStandupReport(t *testing.T) {
	defer TearDown()
	reportStylesSetUp("")

	report, err := generateUserCardsStandupReport(reportStylesConfig(), reportStylesStandups(), []string{"carol"}, nil, "channel_id", reportStylesDate)
	assert.NoError(t, err)

	assert.Equal(t, config.PostTypeStandupReport, report.Type)

	compact, err := generateUserCompactStandupReport(reportStylesConfig(), reportStylesStandups(), []string{"carol"}, nil, "channel_id", reportStylesDate)
	assert.NoError(t, err)
	assert.Equal(t, compact.Message, report.Message,
		"the message is the compact report, for search, mentions and clients without the webapp")

	prop, ok := report.GetProp(PostPropStandupReport).(map[string]interface{})
	if !assert.True(t, ok, "the report is carried as a generic map, which gob can send") {
		return
	}

	data, err := json.Marshal(prop)
	assert.NoError(t, err)

	var carried cardsReport
	assert.NoError(t, json.Unmarshal(data, &carried))
	assert.Equal(t, cardsReport{
		Title: "Standup Report for *24 Oct 2026*",
		Notes: "@carol has not submitted their standup.",
		Members: []reportMember{
			{UserID: "user_id_1", DisplayName: "Foo Bar", Sections: []reportSection{
				{Title: "Today", Body: "1. Internal sync with @maria"},
			}},
			{UserID: "user_id_2", DisplayName: "John Doe", Sections: []reportSection{
				{Title: "Today", Body: "1. Release notes"},
				{Title: "Blockers", Body: "1. Too much mail"},
			}},
		},
	}, carried)
}

func TestPostTypeStandupReport_FitsTheDatabase(t *testing.T) {
	// Posts.Type is a VARCHAR(26), and Post.IsValid does not check its length,
	// so a longer type is only refused by the database when the report is saved.
	assert.LessOrEqual(t, len(config.PostTypeStandupReport), 26)
	assert.True(t, strings.HasPrefix(config.PostTypeStandupReport, model.PostCustomTypePrefix))
}

func TestGenerateReport_NewFormats(t *testing.T) {
	formats := map[string]string{
		config.ReportFormatUserCompact:     "",
		config.ReportFormatUserAttachments: model.PostTypeMessageAttachment,
		config.ReportFormatUserCards:       config.PostTypeStandupReport,
	}

	for format, postType := range formats {
		t.Run(format, func(t *testing.T) {
			defer TearDown()
			reportStylesSetUp("")

			standupConfig := reportStylesConfig()
			standupConfig.ReportFormat = format

			post, err := generateReport(standupConfig, reportStylesStandups(), nil, nil, "channel_id", reportStylesDate)
			assert.NoError(t, err)
			if assert.NotNil(t, post) {
				assert.Equal(t, postType, post.Type)
			}
		})
	}
}
