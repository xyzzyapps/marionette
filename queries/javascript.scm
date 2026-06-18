;; JavaScript comment extraction queries for TagDoc
;; Note: The `.` (anchor) operator is not supported in this tree-sitter version.
;; Matching is proximity-based with Go-side @tag-block/@tag-inline filtering.

((comment)+ @comment
  (function_declaration
    name: (identifier) @entity_name
  ) @entity)

((comment)+ @comment
  (variable_declarator
    name: (identifier) @entity_name
    value: (arrow_function)
  ) @entity)

((comment)+ @comment
  (class_declaration
    name: (identifier) @entity_name
  ) @entity)

((comment)+ @comment
  (method_definition
    name: (property_identifier) @entity_name
  ) @entity)

((comment)+ @comment
  (expression_statement) @entity)

((comment)+ @comment
  (lexical_declaration) @entity)

((comment)+ @comment
  (return_statement) @entity)
