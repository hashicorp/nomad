import AdapterError from '@ember-data/adapter/error';

export const NO_LEADER = 'No cluster leader';

export default AdapterError.extend({
  message: NO_LEADER,
});
