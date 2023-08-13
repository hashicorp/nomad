/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check
import { default as FlashService } from 'ember-cli-flash/services/flash-messages';
import { action } from '@ember/object';

/**
 * @typedef {Object} NotificationObject
 * @property {string} title
 * @property {string} [message]
 * @property {string} [type]
 * @property {string} [color = 'neutral']
 * @property {boolean} [sticky = true]
 * @property {boolean} [destroyOnClick = false]
 * @property {number} [timeout = 5000]
 */

/**
 * @class NotificationsService
 * @extends FlashService
 * A wrapper service around Ember Flash Messages, for adding notifications to the UI
 */
export default class NotificationsService extends FlashService {
  /**
   * @param {NotificationObject} notificationObject
   * @returns {FlashService}
   */
  add(notificationObject) {
    // Set some defaults
    if (!('type' in notificationObject)) {
      notificationObject.type = notificationObject.color || 'neutral';
    }

    if (!('destroyOnClick' in notificationObject)) {
      notificationObject.destroyOnClick = false;
    }

    if (!('timeout' in notificationObject)) {
      notificationObject.timeout = 5000;
    }

    return super.add(notificationObject);
  }

  /**
   * Clears all notifications
   * @returns {FlashService}
   */
  @action
  clearAll() {
    return super.clearMessages();
  }
}
