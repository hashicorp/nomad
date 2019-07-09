import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { computed } from '@ember/object';
import hclToJson from 'hcl-to-json';

export default Model.extend({
  name: attr('string'),
  description: attr('string'),
  rules: attr('string'),

  // FIXME remove if/when API can return rules in JSON
  rulesJson: computed('rules', function() {
    try {
      return hclToJson(this.get('rules'));
    } catch (e) {
      return null;
    }
  }),
});
