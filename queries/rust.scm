;; Rust comment extraction queries for TagDoc
;; Note: Rust uses `line_comment` and `block_comment` node types (not `comment`).

((line_comment)+ @comment
  (function_item
    name: (identifier) @entity_name
  ) @entity)

((line_comment)+ @comment
  (struct_item
    name: (type_identifier) @entity_name
  ) @entity)

((line_comment)+ @comment
  (trait_item
    name: (type_identifier) @entity_name
  ) @entity)

((line_comment)+ @comment
  (impl_item) @entity)

((line_comment)+ @comment
  (enum_item
    name: (type_identifier) @entity_name
  ) @entity)

((line_comment)+ @comment
  (const_item
    name: (identifier) @entity_name
  ) @entity)

((line_comment)+ @comment
  (static_item
    name: (identifier) @entity_name
  ) @entity)

((line_comment)+ @comment
  (expression_statement) @entity)

((line_comment)+ @comment
  (let_declaration) @entity)

((line_comment)+ @comment
  (macro_invocation) @entity)
