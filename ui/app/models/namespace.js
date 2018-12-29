import { readOnly } from '@ember/object/computed';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export default Model.extend({
  name: readOnly('id'),
  hash: attr('string'),
  description: attr('string'),
});
