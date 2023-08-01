module.exports = {
  ...require('@hashicorp/platform-cli/config/stylelint.config'),
  rules: {
    'selector-pseudo-class-no-unknown': [
      true,
      {
        ignorePseudoClasses: ['first', 'last'],
      },
    ],
  },
}
