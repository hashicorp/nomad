import Component from '@ember/component';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('page-layout')
export default class PageLayout extends Component {
  isGutterOpen = false;
}
