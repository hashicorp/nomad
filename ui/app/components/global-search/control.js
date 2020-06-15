import Component from '@ember/component';
import { tagName } from '@ember-decorators/component';
import { task } from 'ember-concurrency';
import { action } from '@ember/object';

@tagName('')
export default class GlobalSearchControl extends Component {
  @task(function*() {})
  search;

  @action select() {}

  calculatePosition(trigger) {
    const { top, left, width } = trigger.getBoundingClientRect();
    return {
      style: {
        left,
        width,
        top,
      },
    };
  }
}
