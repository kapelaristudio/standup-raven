package notification

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/standup-raven/standup-raven/server/config"
	"github.com/standup-raven/standup-raven/server/logger"
	"github.com/standup-raven/standup-raven/server/otime"
	"github.com/standup-raven/standup-raven/server/standup"
	"github.com/standup-raven/standup-raven/server/util"
)

// PostPropStandupReport is the post prop that carries a ReportFormatUserCards
// report for the webapp to draw.
const PostPropStandupReport = "standup_report"

// reportSection is one section of one member's standup, already rendered to
// Markdown the way every report format renders it.
type reportSection struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// reportMember is one member's standup in a report that groups by member.
type reportMember struct {
	UserID      string          `json:"user_id"`
	DisplayName string          `json:"display_name"`
	Sections    []reportSection `json:"sections"`
}

// cardsReport is what a ReportFormatUserCards post carries in its props. The
// post's message holds the same report as Markdown, which is what search, the
// mobile apps and mention notifications see.
type cardsReport struct {
	Title   string         `json:"title"`
	Notes   string         `json:"notes"`
	Members []reportMember `json:"members"`
}

// reportMembers collects the sections each member filled in, skipping the ones
// they left empty, in the order the standup configures them.
func reportMembers(standupConfig *standup.Config, userStandups []*standup.UserStandup) ([]reportMember, error) {
	members := make([]reportMember, 0, len(userStandups))

	for _, userStandup := range userStandups {
		displayName, err := getUserDisplayName(userStandup.UserID)
		if err != nil {
			logger.Debug("Couldn't fetch display name for user", err, map[string]string{"userID": userStandup.UserID})
			return nil, err
		}

		member := reportMember{UserID: userStandup.UserID, DisplayName: displayName, Sections: []reportSection{}}

		for _, sectionTitle := range standupConfig.Sections {
			lines := userStandup.Standup[sectionTitle]
			if lines == nil || len(*lines) == 0 {
				continue
			}

			member.Sections = append(member.Sections, reportSection{
				Title: sectionTitle,
				Body:  standup.SectionBody(standupConfig.SectionType(sectionTitle), *lines),
			})
		}

		members = append(members, member)
	}

	return members, nil
}

func reportTitle(date otime.OTime) string {
	return fmt.Sprintf("Standup Report for *%s*", date.Format("2 Jan 2006"))
}

// reportNotes says who has not submitted and who is away, mentioning them as
// the user aggregated report does, so that they are notified.
func reportNotes(membersNoStandup, membersOutOfOffice []string) string {
	var notes []string

	if len(membersNoStandup) > 0 {
		notes = append(notes, fmt.Sprintf("@%s %s not submitted their standup.", strings.Join(membersNoStandup, ", @"), util.HasHave(len(membersNoStandup))))
	}

	if len(membersOutOfOffice) > 0 {
		notes = append(notes, fmt.Sprintf("@%s %s out of office.", strings.Join(membersOutOfOffice, ", @"), util.IsAre(len(membersOutOfOffice))))
	}

	return strings.Join(notes, "\n\n")
}

// memberMarkdown is one member's sections with the section titles in bold,
// which reads as a label in a card and keeps the heading levels for the names.
func memberMarkdown(member reportMember) string {
	parts := make([]string, 0, len(member.Sections))
	for _, section := range member.Sections {
		parts = append(parts, fmt.Sprintf("**%s**\n%s", section.Title, section.Body))
	}

	return strings.Join(parts, "\n\n")
}

// compactMarkdown is the whole report without profile pictures: the title, the
// notes, and each member under their name, set apart by a rule.
func compactMarkdown(title, notes string, members []reportMember) string {
	text := "#### " + title + "\n"

	if len(members) == 0 {
		return text + "\n:warning: **No user has submitted their standup.**"
	}

	if notes != "" {
		text += "\n" + notes + "\n"
	}

	for _, member := range members {
		text += "\n---\n#### " + member.DisplayName + "\n" + memberMarkdown(member) + "\n"
	}

	return text
}

// generateUserCompactStandupReport groups by member like the user aggregated
// report, without the Markdown profile pictures that Mattermost can only show
// as small square images on a line of their own.
func generateUserCompactStandupReport(
	standupConfig *standup.Config,
	userStandups []*standup.UserStandup,
	membersNoStandup []string,
	membersOutOfOffice []string,
	channelID string,
	date otime.OTime,
) (*model.Post, error) {
	logger.Debug("Generating user compact standup report for channel: "+channelID, nil)

	members, err := reportMembers(standupConfig, userStandups)
	if err != nil {
		return nil, err
	}

	return &model.Post{
		ChannelId: channelID,
		UserId:    config.GetConfig().BotUserID,
		Message:   compactMarkdown(reportTitle(date), reportNotes(membersNoStandup, membersOutOfOffice), members),
	}, nil
}

// generateUserAttachmentsStandupReport puts each member in a Mattermost message
// attachment, with their name and profile picture in its author line.
// Mattermost notifies mentions in attachments, but its search only looks at
// the message, so the members' standups are not found by searching.
func generateUserAttachmentsStandupReport(
	standupConfig *standup.Config,
	userStandups []*standup.UserStandup,
	membersNoStandup []string,
	membersOutOfOffice []string,
	channelID string,
	date otime.OTime,
) (*model.Post, error) {
	logger.Debug("Generating user attachments standup report for channel: "+channelID, nil)

	members, err := reportMembers(standupConfig, userStandups)
	if err != nil {
		return nil, err
	}

	text := "#### " + reportTitle(date) + "\n"
	if len(members) == 0 {
		text += "\n:warning: **No user has submitted their standup.**"
	} else if notes := reportNotes(membersNoStandup, membersOutOfOffice); notes != "" {
		text += "\n" + notes
	}

	post := &model.Post{
		ChannelId: channelID,
		UserId:    config.GetConfig().BotUserID,
		Message:   text,
	}

	attachments := make([]*model.MessageAttachment, 0, len(members))
	for _, member := range members {
		body := memberMarkdown(member)
		attachments = append(attachments, &model.MessageAttachment{
			Fallback:   member.DisplayName + "\n" + body,
			AuthorName: member.DisplayName,
			AuthorIcon: util.UserIconURL(member.UserID),
			Text:       body,
		})
	}

	if len(attachments) > 0 {
		model.ParseMessageAttachment(post, attachments)
	}

	return post, nil
}

// generateUserCardsStandupReport is drawn by the webapp as a card per member
// with their round profile picture. The message is the compact report, which is
// what search, mention notifications and clients without the plugin's webapp
// (the mobile apps, an ephemeral copy of the report) work with.
func generateUserCardsStandupReport(
	standupConfig *standup.Config,
	userStandups []*standup.UserStandup,
	membersNoStandup []string,
	membersOutOfOffice []string,
	channelID string,
	date otime.OTime,
) (*model.Post, error) {
	logger.Debug("Generating user cards standup report for channel: "+channelID, nil)

	members, err := reportMembers(standupConfig, userStandups)
	if err != nil {
		return nil, err
	}

	report := cardsReport{
		Title:   reportTitle(date),
		Notes:   reportNotes(membersNoStandup, membersOutOfOffice),
		Members: members,
	}

	post := &model.Post{
		ChannelId: channelID,
		UserId:    config.GetConfig().BotUserID,
		Type:      config.PostTypeStandupReport,
		Message:   compactMarkdown(report.Title, report.Notes, report.Members),
	}
	// Plugin API calls reach the server over gob, which only carries the
	// generic map and slice types in props, so the report goes as those.
	prop, err := genericJSON(report)
	if err != nil {
		return nil, err
	}
	post.AddProp(PostPropStandupReport, prop)

	return post, nil
}

func genericJSON(value interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var generic map[string]interface{}
	if err := json.Unmarshal(data, &generic); err != nil {
		return nil, err
	}

	return generic, nil
}
