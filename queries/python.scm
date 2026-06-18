;; Python comment extraction queries for TagDoc
;; Note: The `.` (anchor) operator is not supported in this tree-sitter version.

((comment)+ @comment
  (function_definition
    name: (identifier) @entity_name
  ) @entity)

((comment)+ @comment
  (class_definition
    name: (identifier) @entity_name
  ) @entity)

((comment)+ @comment
  (decorator)
  (function_definition
    name: (identifier) @entity_name
  ) @entity)

((comment)+ @comment
  (expression_statement) @entity)

((comment)+ @comment
  (assignment) @entity)
