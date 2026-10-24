import * as React from 'react';
import PropTypes from 'prop-types';

import './style.css';

// Text from the report goes through the web app's own Markdown rendering, so
// mentions, emoji and links look and behave as they do in any other post.
function markdown(text) {
    const {formatText, messageHtmlToComponent} = window.PostUtils;
    return messageHtmlToComponent(formatText(text, {atMentions: true}));
}

function avatarURL(siteURL, userID, profiles) {
    const lastPictureUpdate = (profiles[userID] || {}).last_picture_update || 0;
    return `${siteURL}/api/v4/users/${userID}/image?_=${lastPictureUpdate}`;
}

// Draws a standup report posted in the "user cards" format: a card per member
// with their round profile picture, as Markdown cannot. The post's message holds
// the same report as Markdown, which is shown if the report itself is missing.
function StandupReport({post, siteURL = '', profiles = {}}) {
    const report = (post.props || {}).standup_report;

    if (!report || !Array.isArray(report.members)) {
        return <div className={'standup-report-fallback'}>{markdown(post.message || '')}</div>;
    }

    return (
        <div className={'standup-report'}>
            <div className={'standup-report-title'}>{markdown(report.title || '')}</div>
            {report.notes ? <div className={'standup-report-notes'}>{markdown(report.notes)}</div> : null}
            {report.members.length === 0 ? (
                <div className={'standup-report-notes'}>{markdown(':warning: **No user has submitted their standup.**')}</div>
            ) : null}
            {report.members.map((member) => (
                <section
                    key={member.user_id}
                    className={'standup-report-card'}
                    aria-label={member.display_name}
                >
                    <header className={'standup-report-member'}>
                        <img
                            className={'standup-report-avatar'}
                            src={avatarURL(siteURL, member.user_id, profiles)}
                            alt={''}
                        />
                        <span className={'standup-report-name'}>{member.display_name}</span>
                    </header>
                    {(member.sections || []).map((section) => (
                        <div
                            key={section.title}
                            className={'standup-report-section'}
                        >
                            <div className={'standup-report-section-title'}>{section.title}</div>
                            <div className={'standup-report-section-body'}>{markdown(section.body || '')}</div>
                        </div>
                    ))}
                </section>
            ))}
        </div>
    );
}

StandupReport.propTypes = {
    post: PropTypes.shape({
        message: PropTypes.string,
        props: PropTypes.object,
    }).isRequired,
    siteURL: PropTypes.string,
    profiles: PropTypes.object,
};

export default StandupReport;
