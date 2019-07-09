import { Ability } from 'ember-can';
import { inject as service } from '@ember/service';
import { equal } from '@ember/object/computed';

export default Ability.extend({
  token: service(),

  canRun: equal('token.selfToken.type', 'management'),
});
