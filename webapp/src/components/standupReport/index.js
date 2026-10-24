import {connect} from 'react-redux';

import util from '../../utils';

import StandupReport from './standupReport';

const mapStateToProps = (state) => ({
    siteURL: util.trimTrailingSlash(util.getValueSafely(state, 'entities.general.config.SiteURL', '')),
    profiles: util.getValueSafely(state, 'entities.users.profiles', {}),
});

export default connect(mapStateToProps)(StandupReport);
