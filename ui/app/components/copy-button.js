import Component from '@ember/component';
import { task, timeout } from 'ember-concurrency';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('copy-button')
export default class CopyButton extends Component {
  clipboardText = null;
  state = null;

  @(task(function* () {
    this.set('state', 'success');

    yield timeout(2000);
    this.set('state', null);
  }).restartable())
  indicateSuccess;
}
