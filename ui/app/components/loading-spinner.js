import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';

export default class LoadingSpinner extends Component {
  @tracked paused = false;
}
