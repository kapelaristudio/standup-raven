import React from 'react';
import PropTypes from 'prop-types';
import ChannelHeaderButtonIcon from './components/channelHeaderButton';
import reducer from './reducer';
import Actions from './actions';
import StandupModal from './components/standupModal';
import ConfigModal from './components/configModal';
import StandupReport from './components/standupReport';
import Constants from './constants';
import * as Sentry from '@sentry/browser';
import SentryBoundary, {withBoundary} from './SentryBoundary';
import utils from './utils';
import * as RavenClient from './raven-client';

class StandupRavenPlugin {
    // eslint-disable-next-line class-methods-use-this
    async initialize(registry, store) {
        const siteURL = utils.getValueSafely(store.getState(), 'entities.general.config.SiteURL');
        const pluginConfig = await RavenClient.Config.getPluginConfig(siteURL);

        if (pluginConfig.enableErrorReporting) {
            initSentry(pluginConfig.sentryWebappDSN);
        }

        registry.registerChannelHeaderButtonAction(
            <ChannelHeaderButtonIcon/>,
            (channel) => store.dispatch(Actions.openStandupModal(channel.id)),
            Constants.PLUGIN_DISPLAY_NAME,
            Constants.PLUGIN_DISPLAY_NAME,
        );

        registry.registerRootComponent(withBoundary(StandupModal));
        registry.registerRootComponent(withBoundary(ConfigModal));
        registry.registerPostTypeComponent(Constants.POST_TYPE_STANDUP_REPORT, GuardedStandupReport);
        registry.registerWebSocketEventHandler(
            `custom_${Constants.PLUGIN_NAME}_open_config_modal`,
            () => {
                store.dispatch(Actions.openConfigModal());
            },
        );
        registry.registerWebSocketEventHandler(
            `custom_${Constants.PLUGIN_NAME}_open_standup_modal`,
            (event) => {
                // The prompt sends the channel it belongs to, which is not
                // necessarily the channel the user is looking at.
                store.dispatch(Actions.openStandupModal(utils.getValueSafely(event, 'data.channel_id')));
            },
        );

        registry.registerWebSocketEventHandler(
            `custom_${Constants.PLUGIN_NAME}_add_active_channel`,
            (event) => {
                store.dispatch(Actions.addActiveChannel(event.data.channel_id));
            },
        );

        registry.registerWebSocketEventHandler(
            `custom_${Constants.PLUGIN_NAME}_remove_active_channel`,
            (event) => {
                store.dispatch(Actions.removeActiveChannel(event.data.channel_id));
            },
        );

        registry.registerReducer(reducer);
    }
}

// A report that fails to draw falls back to its message as plain text, in place
// in the channel, rather than to the boundary's usual message over the page.
function GuardedStandupReport(props) {
    const fallback = <div className={'standup-report-fallback'}>{props.post.message}</div>;

    return (
        <SentryBoundary fallback={fallback}>
            <StandupReport {...props}/>
        </SentryBoundary>
    );
}

GuardedStandupReport.propTypes = {
    post: PropTypes.shape({message: PropTypes.string}).isRequired,
};

function initSentry(dsn) {
    Sentry.init({
        dsn,
    });

    Sentry.getCurrentScope().setTag('pluginComponent', 'webapp');
}

window.registerPlugin(Constants.PLUGIN_NAME, new StandupRavenPlugin());
