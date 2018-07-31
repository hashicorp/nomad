import { AdapterError } from 'ember-data/adapters/errors';

export const NO_LEADER = 'No cluster leader';

export default AdapterError.extend({
  message: NO_LEADER,
});
