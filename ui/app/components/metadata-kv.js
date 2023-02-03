import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';

export default class MetadataKvComponent extends Component {
  editing = false;
  @tracked value = this.args.value;
}
