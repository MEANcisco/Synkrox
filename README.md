# Herramienta de Sincronización Firebird a API

## Descripción
Esta aplicación en Go está diseñada para sincronizar datos de productos entre una base de datos Firebird y un endpoint de API REST. La herramienta verifica periódicamente los cambios en la base de datos Firebird, sube imágenes de productos si están disponibles y envía actualizaciones, creaciones o eliminaciones de productos a un endpoint del servidor local (`localhost:3000/products`). La aplicación también realiza un seguimiento del estado de la sincronización utilizando una base de datos SQLite.

## Características
- **Integración con Firebird y SQLite**: Conecta a una base de datos Firebird para obtener información de productos y utiliza SQLite para realizar un seguimiento del estado de la sincronización.
- **Subida de Imágenes**: Sube imágenes de productos a un endpoint de API especificado si se detectan cambios.
- **Sistema de Notificaciones**: Proporciona notificaciones de escritorio sobre el estado de la sincronización.
- **Icono en la Barra de Tareas**: Muestra el número de acciones de sincronización pendientes o indica que el sistema está inactivo.

## Requisitos Previos
- Go 1.16 o superior
- FirebirdSQL instalado y en ejecución
- SQLite 3 instalado
- API REST disponible en `http://localhost:3000/products`

## Instalación
1. Clona el repositorio:
   ```sh
   git clone <repository-url>
   cd <repository-folder>
   ```

2. Instala las dependencias:
   ```sh
   go get github.com/mattn/go-sqlite3
   go get github.com/nakagami/firebirdsql
   go get github.com/martinlindhe/notify
   ```

## Configuración
Edita las constantes de configuración en el archivo `main.go` según sea necesario:
- **Configuración de la Base de Datos**:
  ```go
  const (
      dbHost       = "<FirebirdHost>"
      dbPort       = "3050"
      dbName       = "<FirebirdDatabasePath>"
      dbUser       = "SYSDBA"
      dbPassword   = "masterkey"
      sqliteDBPath = "synkros.db"
  )
  ```
  Reemplaza `<FirebirdHost>` y `<FirebirdDatabasePath>` con los valores correctos para tu configuración.

## Uso
1. **Ejecuta la aplicación**:
   ```sh
   go run main.go
   ```
   La aplicación se conectará a la base de datos Firebird, verificará los cambios en los datos de productos, sincronizará con la API REST y actualizará la base de datos SQLite en consecuencia.

2. **Ciclo de Sincronización**:
   La aplicación realiza la sincronización cada hora de manera predeterminada. Verifica los cambios en los productos, sube imágenes y envía actualizaciones al endpoint de la API.

## Endpoint de la API REST
La aplicación se comunica con los siguientes endpoints de la API REST:
- `POST /products`: Crear un nuevo producto.
- `PUT /products`: Actualizar un producto existente.
- `DELETE /products`: Eliminar un producto.
- `POST /products/uploadAsset`: Subir imágenes de productos para ser utilizadas por las entradas de productos.

## Registro y Notificaciones
- **Registro**: Los registros se imprimen en la consola para proporcionar información detallada sobre el proceso de sincronización.
- **Notificaciones**: Se envían notificaciones de escritorio utilizando `github.com/martinlindhe/notify`, indicando el estado de la sincronización.

## Manejo de Errores
- Los errores durante la sincronización se registran en la consola.
- En caso de fallo al subir un asset o sincronizar datos de productos, el producto afectado se omite y la aplicación continúa con el siguiente producto.

## Dependencias
- **Driver FirebirdSQL**: `github.com/nakagami/firebirdsql` - Para conectarse a la base de datos Firebird.
- **Driver SQLite**: `github.com/mattn/go-sqlite3` - Para las operaciones de SQLite y seguimiento del estado de sincronización de productos.
- **Librería de Notificaciones**: `github.com/martinlindhe/notify` - Para las notificaciones de escritorio sobre el estado de sincronización.

## Licencia
Este proyecto está licenciado bajo la Licencia MIT - consulta el archivo LICENSE para más detalles.

## Agradecimientos
- `github.com/martinlindhe/notify` - Por proporcionar notificaciones de escritorio de manera sencilla.

## Mejoras Futuras
- Implementar reintentos de errores para intentos de sincronización fallidos.
- Mejorar el sistema de notificaciones para proporcionar información más detallada.
- Agregar opciones de configuración para el intervalo de sincronización y las URLs de los endpoints.

