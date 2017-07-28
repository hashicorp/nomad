import Ember from 'ember';
import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

const { computed } = Ember;

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
  restartReason: attr('string'),
  setupError: attr('string'),
  taskSignalReason: attr('string'),
  validationError: attr('string'),
  vaultError: attr('string'),
  message: attr('string'),

  // The pertinent message for the event will be one of these
  displayMessage: computed.or(
    'message',
    'driverMessage',
    'restartReason',
    'killReason',
    'taskSignalReason',
    'downloadError',
    'driverError',
    'killError',
    'setupError',
    'validationError',
    'vaultError'
  ),
});
