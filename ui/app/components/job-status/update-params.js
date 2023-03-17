// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class JobStatusUpdateParamsComponent extends Component {
  @tracked rawDefinition = null;

  get updateParams() {
    if (this.rawDefinition) {
      return this.rawDefinition.Update;
    } else {
      return null;
    }
  }

  @action async fetchJobDefinition() {
    this.rawDefinition = await this.args.job.fetchRawDefinition();
  }
}
