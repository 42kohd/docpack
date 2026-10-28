# Task: convert an existing page

Follow the rules in SYSTEM.md.

1. Here is the skeleton for the target type, made by `docpack new`: (paste)
2. Here is the existing page, in any format: (paste)

Move the existing content into the skeleton's sections. Where the old page has
a section the template does not allow, fold its content into the nearest
allowed section as a `###` subsection. Turn command examples into
`::: command` blocks with all three operating systems, and turn hand-written
flag or field tables into `{{ flags … }}` or `{{ fields … }}` references.
Replace the product name, binary name and version numbers with placeholders.
Drop anything that is marketing rather than instruction.

Return the complete document.
