import { attr } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default class Lifecycle extends Fragment {
  @fragmentOwner() task;

  @attr('string') hook;
  @attr('boolean') sidecar;
}
