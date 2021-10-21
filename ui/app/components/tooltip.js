import Component from '@glimmer/component';

export default class Tooltip extends Component {
  get text() {
    const inputText = this.args.text;
    if (!inputText || inputText.length < 30) {
      return inputText;
    }

    const prefix = inputText.substr(0, 15).trim();
    const suffix = inputText.substr(inputText.length - 10, inputText.length).trim();
    return `${prefix}...${suffix}`;
  }
}
