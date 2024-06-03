FROM --platform=amd64 golang:1.20 as build
WORKDIR /service_go_fetch_customer
COPY go.mod go.sum ./
COPY main.go .
RUN go build -tags lambda.norpc -o main main.go
FROM public.ecr.aws/lambda/provided:al2023
COPY --from=build /service_go_fetch_customer/main ./main
ENTRYPOINT [ "./main" ]


