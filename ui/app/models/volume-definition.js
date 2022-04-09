import { alias, equal } from '@ember/object/computed';
import { attr } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default class VolumeDefinition extends Fragment {
  @fragmentOwner() taskGroup;

  @attr('string') name;

  @attr('string') source;
  @attr('string') type;
  @attr('boolean') readOnly;

  @equal('type', 'csi') isCSI;
  @alias('taskGroup.job.namespace') namespace;
}
