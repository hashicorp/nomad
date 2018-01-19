import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

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
});
