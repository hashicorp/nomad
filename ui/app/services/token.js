/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service, { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { alias, reads } from '@ember/object/computed';
import { getOwner } from '@ember/application';
import { assign } from '@ember/polyfills';
import { task, timeout } from 'ember-concurrency';
import queryString from 'query-string';
import fetch from 'nomad-ui/utils/fetch';
import classic from 'ember-classic-decorator';
import moment from 'moment';

const MINUTES_LEFT_AT_WARNING = 10;
const EXPIRY_NOTIFICATION_TITLE = 'Your access is about to expire';
@classic
export default class TokenService extends Service {
  @service store;
  @service system;
  @service router;
  @service notifications;

  aclEnabled = true;

  tokenNotFound = false;

  @computed
  get secret() {
    return window.localStorage.nomadTokenSecret;
  }

  set secret(value) {
    if (value == null) {
      window.localStorage.removeItem('nomadTokenSecret');
    } else {
      window.localStorage.nomadTokenSecret = value;
    }
  }

  @task(function* () {
    const TokenAdapter = getOwner(this).lookup('adapter:token');
    try {
      var token = yield TokenAdapter.findSelf();
      this.secret = token.secret;
      return token;
    } catch (e) {
      const errors = e.errors ? e.errors.mapBy('detail') : [];
      if (errors.find((error) => error === 'ACL support disabled')) {
        this.set('aclEnabled', false);
      }
      if (errors.find((error) => error === 'ACL token not found')) {
        this.set('tokenNotFound', true);
      }
      return null;
    }
  })
  fetchSelfToken;

  @reads('fetchSelfToken.lastSuccessful.value') selfToken;

  async exchangeOneTimeToken(oneTimeToken) {
    const TokenAdapter = getOwner(this).lookup('adapter:token');

    const token = await TokenAdapter.exchangeOneTimeToken(oneTimeToken);
    this.secret = token.secret;
  }

  @task(function* () {
    try {
      if (this.selfToken) {
        return yield this.selfToken.get('policies');
      } else {
        let policy = yield this.store.findRecord('policy', 'anonymous');
        return [policy];
      }
    } catch (e) {
      return [];
    }
  })
  fetchSelfTokenPolicies;

  @alias('fetchSelfTokenPolicies.lastSuccessful.value') selfTokenPolicies;

  @task(function* () {
    yield this.fetchSelfToken.perform();
    this.kickoffTokenTTLMonitoring();
    if (this.aclEnabled) {
      yield this.fetchSelfTokenPolicies.perform();
    }
  })
  fetchSelfTokenAndPolicies;

  // All non Ember Data requests should go through authorizedRequest.
  // However, the request that gets regions falls into that category.
  // This authorizedRawRequest is necessary in order to fetch data
  // with the guarantee of a token but without the automatic region
  // param since the region cannot be known at this point.
  authorizedRawRequest(url, options = {}) {
    const credentials = 'include';
    const headers = {};
    const token = this.secret;

    if (token) {
      headers['X-Nomad-Token'] = token;
    }

    return fetch(url, assign(options, { headers, credentials }));
  }

  authorizedRequest(url, options) {
    if (this.get('system.shouldIncludeRegion')) {
      const region = this.get('system.activeRegion');
      if (region && url.indexOf('region=') === -1) {
        url = addParams(url, { region });
      }
    }

    return this.authorizedRawRequest(url, options);
  }

  reset() {
    this.fetchSelfToken.cancelAll({ resetState: true });
    this.fetchSelfTokenPolicies.cancelAll({ resetState: true });
    this.fetchSelfTokenAndPolicies.cancelAll({ resetState: true });
    this.monitorTokenTime.cancelAll({ resetState: true });
    window.localStorage.removeItem('nomadOIDCNonce');
    window.localStorage.removeItem('nomadOIDCAuthMethod');
  }

  kickoffTokenTTLMonitoring() {
    this.monitorTokenTime.perform();
  }

  @task(function* () {
    while (this.selfToken?.expirationTime) {
      const diff = new Date(this.selfToken.expirationTime) - new Date();
      // Let the user know at the 10 minute mark,
      // or any time they refresh with under 10 minutes left
      if (diff < 1000 * 60 * MINUTES_LEFT_AT_WARNING) {
        const existingNotification = this.notifications.queue?.find(
          (m) => m.title === EXPIRY_NOTIFICATION_TITLE
        );
        // For the sake of updating the "time left" message, we keep running the task down to the moment of expiration
        if (diff > 0) {
          if (existingNotification) {
            existingNotification.set(
              'message',
              `Your token access expires ${moment(
                this.selfToken.expirationTime
              ).fromNow()}`
            );
          } else {
            if (!this.expirationNotificationDismissed) {
              this.notifications.add({
                title: EXPIRY_NOTIFICATION_TITLE,
                message: `Your token access expires ${moment(
                  this.selfToken.expirationTime
                ).fromNow()}`,
                color: 'warning',
                sticky: true,
                customCloseAction: () => {
                  this.set('expirationNotificationDismissed', true);
                },
                customAction: {
                  label: 'Re-authenticate',
                  action: () => {
                    this.router.transitionTo('settings.tokens');
                  },
                },
              });
            }
          }
        } else {
          if (existingNotification) {
            existingNotification.setProperties({
              title: 'Your access has expired',
              message: `Your token will need to be re-authenticated`,
            });
          }
          this.monitorTokenTime.cancelAll(); // Stop updating time left after expiration
        }
      }
      yield timeout(1000);
    }
  })
  monitorTokenTime;
}

function addParams(url, params) {
  const paramsStr = queryString.stringify(params);
  const delimiter = url.includes('?') ? '&' : '?';
  return `${url}${delimiter}${paramsStr}`;
}
