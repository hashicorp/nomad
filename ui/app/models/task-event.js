import Ember from 'ember';
import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';
import moment from 'moment';

const { computed } = Ember;
const displayProps = [
  'message',
  'validationError',
  'setupError',
  'driverError',
  'downloadError',
  'killReason',
  'killTimeout',
  'killError',
  'exitCode',
  'signal',
  'startDelay',
  'restartReason',
  'failedSibling',
  'taskSignal',
  'taskSignalReason',
  'driverMessage',
];

export default Fragment.extend({
  state: fragmentOwner(),

  type: attr('string'),
  signal: attr('number'),
  exitCode: attr('number'),

  time: attr('date'),
  timeNanos: attr('number'),

  downloadError: attr('string'),
  driverError: attr('string'),
  driverMessage: attr('string'),
  killError: attr('string'),
  killReason: attr('string'),
  killTimeout: attr('number'),
  restartReason: attr('string'),
  setupError: attr('string'),
  startDelay: attr('number'),
  taskSignal: attr('string'),
  taskSignalReason: attr('string'),
  validationError: attr('string'),
  vaultError: attr('string'),
  message: attr('string'),
  failedSibling: attr('string'),

  displayMessage: computed(...displayProps, function() {
    let desc = '';
    switch (this.get('type')) {
      case 'Task Setup':
        desc = this.get('message');
        break;
      case 'Started':
        desc = 'Task started by client';
        break;
      case 'Received':
        desc = 'Task received by client';
        break;
      case 'Failed Validation':
        desc = this.get('validationError') || 'Validation of task failed';
        break;
      case 'Setup Failure':
        desc = this.get('setupError') || 'Task setup failed';
        break;
      case 'Driver Failure':
        desc = this.get('driverError') || 'Failed to start task';
        break;
      case 'Downloading Artifacts':
        desc = 'Client is downloading artifacts';
        break;
      case 'Failed Artifact Download':
        desc = this.get('downloadError') || 'Failed to download artifacts';
        break;
      case 'Killing':
        desc =
          this.get('killReason') ||
          (this.get('killTimeout') &&
            `Sent interrupt. Waiting ${this.get('killTimeout')} before force killing`);
        break;
      case 'Killed':
        desc = this.get('killError') || 'Task successfully killed';
        break;
      case 'Terminated':
        var parts = [`Exit Code: ${this.get('exitCode')}`];
        if (this.get('signal')) {
          parts.push(`Signal: ${this.get('signal')}`);
        }
        if (this.get('message')) {
          parts.push(`Exit Message: ${this.get('message')}`);
        }
        desc = parts.join(', ');
        break;
      case 'Restarting':
        var timerMessage = `Task restarting in ${moment
          .duration(this.get('startDelay') / 1000000, 'ms')
          .humanize()}`;
        if (this.get('restartReason') && this.get('restartReason') !== 'Restart within policy') {
          desc = `${this.get('restartReason')} - ${timerMessage}`;
        } else {
          desc = timerMessage;
        }
        break;
      case 'Not Restarting':
        desc = this.get('restartReason') || 'Task exceeded restart policy';
        break;
      case 'Sibling Task Failed':
        desc = this.get('failedSibling')
          ? `Task's sibling ${this.get('failedSibling')} failed`
          : "Task's sibling failed";
        break;
      case 'Signaling':
        var signal = this.get('taskSignal');
        var reason = this.get('taskSignalReason');

        if (!signal && !reason) {
          desc = 'Task being sent a signal';
        } else if (!signal) {
          desc = reason;
        } else if (!reason) {
          desc = `Task being sent signal ${signal}`;
        } else {
          desc = `Task being sent signal ${signal}: ${reason}`;
        }

        break;
      case 'Restart Signaled':
        desc = this.get('restartReason') || 'Task signaled to restart';
        break;
      case 'Driver':
        desc = this.get('driverMessage');
        break;
      case 'Leader Task Dead':
        desc = 'Leader Task in Group dead';
        break;
      case 'Generic':
        desc = this.get('message');
        break;
    }

    return desc;
  }),
});
