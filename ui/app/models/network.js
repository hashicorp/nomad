import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import { array } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  device: attr('string'),
  cidr: attr('string'),
  ip: attr('string'),
  mbits: attr('number'),
  reservedPorts: array(),
  dynamicPorts: array(),
});
