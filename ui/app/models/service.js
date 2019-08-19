import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import { array } from 'ember-data-model-fragments/attributes';
import { computed } from '@ember/object';

export default Fragment.extend({
  name: attr('string'),
  portLabel: attr('number'),
  tags: array({ defaultValue: () => [] }),

  // FIXME service-row instead?
  tagsString: computed('tags.[]', function() {
    return this.get('tags')
      .toArray()
      .join(', ');
  }),
});
