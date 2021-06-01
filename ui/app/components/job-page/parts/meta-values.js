import Component from '@ember/component';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import classic from 'ember-classic-decorator';

import Sortable from 'nomad-ui/mixins/sortable';

@classic
export default class MetaValues extends Component {
  job = null;
  definition = null;

  init() {
    super.init(...arguments);

    // Note: The JOB route does not fetch the raw definition, so we do it here
    this.job.fetchRawDefinition().then(def => this.set('definition', def));
  }

  @computed('definition.Meta', 'job.parameterizedDetails')
  get paramMap() {
    let params = this.job.parameterizedDetails || {};
    let merged = [];

    let required = params.MetaRequired || [];
    let optional = params.MetaOptional || [];

    // Helper for getting the value for a meta, if provided.
    let getValue = name => (this.definition ? this.definition.Meta[name] : '');

    // Merge all of the different info
    let mergeFun = required => name =>
      merged.push({
        name: name,
        value: getValue(name),
        required: required,
      });

    required.forEach(mergeFun(true));
    optional.forEach(mergeFun(false));

    return merged;
  }
}
