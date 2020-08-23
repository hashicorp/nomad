import classic from 'ember-classic-decorator';
import Fragment from 'ember-data-model-fragments/fragment';
import { get, computed } from '@ember/object';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';
import { fragment } from 'ember-data-model-fragments/attributes';

@classic
export default class NodeDriver extends Fragment {
  @fragmentOwner() node;

  @fragment('node-attributes') attributes;

  @computed('name', 'attributes.attributesStructured')
  get attributesShort() {
    const attributes = this.get('attributes.attributesStructured');
    return get(attributes, `driver.${this.name}`);
  }

  @attr('string') name;
  @attr('boolean', { defaultValue: false }) detected;
  @attr('boolean', { defaultValue: false }) healthy;
  @attr('string') healthDescription;
  @attr('date') updateTime;

  @computed('healthy')
  get healthClass() {
    return this.healthy ? 'running' : 'failed';
  }
}
