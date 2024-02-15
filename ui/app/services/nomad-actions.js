/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Service from '@ember/service';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { base64DecodeString } from '../utils/encode';
import config from 'nomad-ui/config/environment';

export default class NomadActionsService extends Service {
  @service can;
  @service store;
  @service token;

  // Note: future Actions Governance work (https://github.com/hashicorp/nomad/issues/18800)
  // will require this to be a computed property that depends on the current user's permissions.
  // For now, we simply check alloc exec privileges.
  get hasActionPermissions() {
    return this.can.can('exec allocation');
  }

  @tracked flyoutActive = false;

  @action openFlyout() {
    this.flyoutActive = true;
  }
  @action closeFlyout() {
    this.flyoutActive = false;
  }

  /**
   * @type {import('../models/action-instance').default[]}
   */
  @tracked
  actionsQueue = [];

  updateQueue() {
    this.actionsQueue = [...this.actionsQueue];
  }

  get runningActions() {
    return this.actionsQueue.filter((a) => a.state === 'running');
  }

  get finishedActions() {
    return this.actionsQueue.filter(
      (a) => a.state === 'complete' || a.state === 'error'
    );
  }

  /**
   * @typedef {Object} RunActionParams
   * @property {import("../models/action").default} action
   * @property {string} allocID
   * @property {string} [peerID]
   */

  /**
   * @param {RunActionParams} params
   */
  @action runAction({ action, allocID, peerID = null }) {
    const job = action.task.taskGroup.job;

    const actionQueueID = `${action.name}-${allocID}-${Date.now()}`;
    /**
     * @type {import ('../models/action-instance').default}
     */
    const actionInstance = this.store.createRecord('action-instance', {
      state: 'pending',
      id: actionQueueID,
      allocID,
      peerID,
    });

    // Note: setting post-createRecord because of a noticed bug
    // when passing action as a property to createRecord.
    actionInstance.set('action', action);

    let wsURL = job.getActionSocketUrl(action, allocID, actionInstance);

    this.establishInstanceSocket(actionInstance, wsURL);

    this.actionsQueue.unshift(actionInstance); // add to the front of the queue
    this.updateQueue();
    this.openFlyout();
  }

  /**
   * @param {import('../models/action').default} action
   */
  @action runActionOnRandomAlloc(action) {
    let allocID =
      action.allocations[Math.floor(Math.random() * action.allocations.length)]
        .id;
    this.runAction({ action, allocID });
  }

  /**
   * @param {import('../models/action').default} action
   */
  @action runActionOnAllAllocs(action) {
    // Generate a new peer ID for these action instances to use
    const peerID = `${action.name}-${Date.now()}`;
    action.allocations.forEach((alloc) => {
      this.runAction({ action, allocID: alloc.id, peerID });
    });
  }

  /**
   * @param {import ('../models/action-instance').default} actionInstance
   */
  @action clearActionInstance(actionInstance) {
    // if instance is still running, stop it
    if (actionInstance.state === 'running') {
      actionInstance.socket.close();
    }
    this.actionsQueue = this.actionsQueue.filter(
      (a) => a.id !== actionInstance.id
    );

    // If action had peers, clear them out as well
    if (actionInstance.peerID) {
      this.actionsQueue = this.actionsQueue.filter(
        (a) => a.peerID !== actionInstance.peerID
      );
    }
    this.updateQueue();
  }

  @action clearFinishedActions() {
    this.actionsQueue = this.actionsQueue.filter((a) => a.state !== 'complete');
  }

  @action stopAll() {
    this.actionsQueue.forEach((a) => {
      if (a.state === 'running') {
        a.socket.close();
      }
    });
    this.updateQueue();
  }

  @action stopPeers(peerID) {
    if (!peerID) {
      return;
    }
    this.actionsQueue
      .filter((a) => a.peerID === peerID)
      .forEach((a) => {
        if (a.state === 'running') {
          a.socket.close();
        }
      });
    this.updateQueue();
  }

  //#region Socket

  get mirageEnabled() {
    return (
      config.environment !== 'production' &&
      config['ember-cli-mirage'] &&
      config['ember-cli-mirage'].enabled !== false
    );
  }

  /**
   * Mocks a WebSocket for testing.
   * @returns {Object}
   */
  createMockWebSocket() {
    let socket = new Object({
      messageDisplayed: false,
      addEventListener: function (event, callback) {
        if (event === 'message') {
          this.onmessage = callback;
        }
        if (event === 'open') {
          this.onopen = callback;
        }
        if (event === 'close') {
          this.onclose = callback;
        }
        if (event === 'error') {
          this.onerror = callback;
        }
      },

      send(e) {
        if (!this.messageDisplayed) {
          this.messageDisplayed = true;
          this.onmessage({
            data: `{"stdout":{"data":"${btoa('Message Received')}"}}`,
          });
        } else {
          this.onmessage({ data: e.replace('stdin', 'stdout') });
        }
      },
    });
    return socket;
  }

  /**
   * Establishes a WebSocket connection for a given action instance.
   *
   * @param {import('../models/action-instance').default} actionInstance - The action instance model.
   * @param {string} wsURL - The WebSocket URL.
   */
  establishInstanceSocket(actionInstance, wsURL) {
    let socket = this.createWebSocket(wsURL);
    actionInstance.set('socket', socket);
    console.log('socket set on', socket);
    // simulate an error
    // socket.error = () => {
    //   socket.onerror();
    // };

    socket.addEventListener('open', () =>
      this.handleSocketOpen(actionInstance, socket)
    );
    socket.addEventListener('message', (event) =>
      this.handleSocketMessage(actionInstance, event)
    );
    socket.addEventListener('close', () =>
      this.handleSocketClose(actionInstance)
    );
    socket.addEventListener('error', () =>
      this.handleSocketError(actionInstance)
    );

    // Open,
    if (this.mirageEnabled) {
      socket.onopen();
      socket.onclose();
    }
  }

  /**
   * Creates a WebSocket or a mock WebSocket for testing.
   *
   * @param {string} wsURL - The WebSocket URL.
   * @returns {WebSocket|Object} - The WebSocket or a mock WebSocket object.
   */
  createWebSocket(wsURL) {
    return this.mirageEnabled
      ? this.createMockWebSocket()
      : new WebSocket(wsURL);
  }

  /**
   * @param {import('../models/action-instance').default} actionInstance - The action instance model.
   * @param {WebSocket} socket - The WebSocket instance.
   */
  handleSocketOpen(actionInstance, socket) {
    actionInstance.state = 'starting';
    actionInstance.createdAt = new Date();

    socket.send(
      JSON.stringify({ version: 1, auth_token: this.token?.secret || '' })
    );
    socket.send(JSON.stringify({ tty_size: { width: 250, height: 100 } }));
  }

  /**
   * @param {import('../models/action-instance').default} actionInstance - The action instance model.
   * @param {MessageEvent} event - The message event.
   */
  handleSocketMessage(actionInstance, event) {
    actionInstance.state = 'running';

    try {
      let jsonData = JSON.parse(event.data);
      if (jsonData.stdout && jsonData.stdout.data) {
        const message = base64DecodeString(jsonData.stdout.data).replace(
          /\x1b\[[0-9;]*[a-zA-Z]/g,
          ''
        );
        actionInstance.messages += '\n' + message;
      } else if (jsonData.stderr && jsonData.stderr.data) {
        actionInstance.state = 'error';
        actionInstance.error += '\n' + base64DecodeString(jsonData.stderr.data);
      }
    } catch (e) {
      actionInstance.state = 'error';
      actionInstance.error += '\n' + e;
    }
  }

  /**
   * Handles the WebSocket 'close' event.
   *
   * @param {import('../models/action-instance').default} actionInstance - The action instance model.
   */
  handleSocketClose(actionInstance) {
    actionInstance.state = 'complete';
    actionInstance.completedAt = new Date();
  }

  /**
   * Handles the WebSocket 'error' event.
   *
   * @param {import('../models/action-instance').default} actionInstance - The action instance model.
   */
  handleSocketError(actionInstance) {
    actionInstance.state = 'error';
    actionInstance.completedAt = new Date();
    actionInstance.error = 'Error connecting to action socket';
  }

  // #endregion Socket
}
