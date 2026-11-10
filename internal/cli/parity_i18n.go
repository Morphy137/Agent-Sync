package cli

// Keep parity strings here so independently developed command groups do not
// need to modify the shared message catalog.
func init() {
	messages[LangEN]["cmd.dest_update"] = "Patch a destination with JSON; [] clears lists and clearTransform:true removes its transform without changing mode."
	messages[LangES]["cmd.dest_update"] = "Actualizar un destino con JSON; [] vacía listas y clearTransform:true quita la transformación sin cambiar el modo."
	messages[LangEN]["cmd.rules_update"] = "Patch a rule with JSON; omitted fields are preserved, [] clears lists, scope repo/local moves it without changing IDs."
	messages[LangES]["cmd.rules_update"] = "Actualizar una regla con JSON; se conservan los campos omitidos, [] vacía listas y scope repo/local la mueve sin cambiar IDs."
	messages[LangEN]["err.patch_input"] = "--input FILE|- is required for a partial JSON update"
	messages[LangES]["err.patch_input"] = "Se requiere --input ARCHIVO|- para una actualización JSON parcial"
}
