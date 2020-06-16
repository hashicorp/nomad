import Component from '@ember/component';
import { classNames, classNameBindings } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('accordion-head')
@classNameBindings('isOpen::is-light', 'isExpandable::is-inactive')
export default class AccordionHead extends Component {
  'data-test-accordion-head' = true;

  buttonLabel = 'toggle';
  isOpen = false;
  isExpandable = true;
  item = null;

  onClose() {}
  onOpen() {}
}
