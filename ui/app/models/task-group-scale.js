import { computed } from '@ember/object';
import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner, fragmentArray } from 'ember-data-model-fragments/attributes';

export default class TaskGroupScale extends Fragment {
  @fragmentOwner() jobScale;

  @attr('string') name;

  @attr('number') desired;
  @attr('number') placed;
  @attr('number') running;
  @attr('number') healthy;
  @attr('number') unhealthy;

  @fragmentArray('scale-event') events;

  @computed('events.length', function() {
    return this.events.length;
  })
  isVisible;
}
