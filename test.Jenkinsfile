podTemplate(containers: [
   containerTemplate(
      name: 'go', image: 'golang:1.16-alpine', ttyEnabled: true, command: 'cat')]
 ){
    node(POD_LABEL) {
        container('go') {
            stage('Test') {
                checkout scm
                withEnv(['CGO_ENABLED=0']) {
                    sh 'go test .'
                }
            }
        }
     }
}
