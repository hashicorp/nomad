import Fragment from 'ember-data-model-fragments/fragment';
import { computed, get } from '@ember/object';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';
import { fragment } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  node: fragmentOwner(),

  attributes: fragment('node-attributes'),

  attributesShort: computed('name', 'attributes.attributesStructured', function() {
    const attributes = this.get('attributes.attributesStructured');
    return get(attributes, `driver.${this.name}`);
  }),

  name: attr('string'),
  detected: attr('boolean', { defaultValue: false }),
  healthy: attr('boolean', { defaultValue: false }),
  healthDescription: attr('string'),
  updateTime: attr('date'),

  healthClass: computed('healthy', function() {
    return this.healthy ? 'running' : 'failed';
  }),
});
