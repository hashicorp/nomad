import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

export default Model.extend({
  diff: attr(),
  failedTGAllocs: fragmentArray('placement-failure', { defaultValue: () => [] }),
});
