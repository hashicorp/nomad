function visitObject(objectIn, handleNode) {
  let objectOut = {}
  Object.keys(objectIn).forEach((key) => {
    objectOut[key] = visitProperty(key, objectIn[key], handleNode)
  })
  return objectOut
}

function visitProperty(key, value, handleNode) {
  const handledValue = handleNode(key, value)
  if (isObject(value)) {
    return visitObject(handledValue, handleNode)
  } else if (isArray(handledValue)) {
    return handledValue.map((v) => visitProperty(key, v, handleNode))
  } else {
    return handledValue
  }
}

// Returns true if a value is an array, false otherwise
export function isArray(value) {
  return value && typeof value === 'object' && value.constructor === Array
}

// Returns true if a value is an object, false otherwise
export function isObject(value) {
  return value && typeof value === 'object' && value.constructor === Object
}

function traverse(valueIn, handleNode = (n) => n) {
  return visitProperty(false, valueIn, handleNode)
}

export default traverse
