import Fragment from 'ember-data-model-fragments/fragment';
import { fragment } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  sidecarService: fragment('sidecar-service'),
});
