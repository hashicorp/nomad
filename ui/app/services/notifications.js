/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import { FlashMessagesService } from 'ember-cli-flash';

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
 * @extends FlashMessagesService
 * A wrapper service around Ember Flash Messages, for adding notifications to the UI
 */
export default class NotificationsService extends FlashMessagesService {
  /**
   * @param {NotificationObject} notificationObject
   * @returns {FlashMessagesService}
   */
  add(notificationObject) {
    const message = /** @type {any} */ (notificationObject.message);

    if (
      message &&
      typeof message === 'object' &&
      typeof message.toHTML !== 'function'
    ) {
      notificationObject.message = message.message || String(message);
    }

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
}
