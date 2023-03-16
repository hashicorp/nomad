// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class JobStatusUpdateParamsComponent extends Component {

  @tracked rawDefinition = null;

  get updateParams() {
    console.log('getupdater', this.rawDefinition);
    if (this.rawDefinition) {
      return this.rawDefinition.Update;
    }
  }

  @action async fetchJobDefinition() {
    this.rawDefinition = await this.args.job.fetchRawDefinition();
  }
}
