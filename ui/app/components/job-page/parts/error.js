import Component from '@ember/component';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class Error extends Component {
  errorMessage = null;
  onDismiss() {}
}
