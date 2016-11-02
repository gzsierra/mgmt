// Mgmt
// Copyright (C) 2013-2016+ James Shubin and the project contributors
// Written by James Shubin <james@shubin.ca> and the project contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package gpg

import (
	"bufio"
	"bytes"
	"crypto"
	"encoding/base64"
	"encoding/gob"
	"io/ioutil"
	"log"
	"os"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
)

func init() {
	gob.Register(&GpgRes{})
}

// GpgRes is a no-op resource that does nothing.
type GpgRes struct {
	Name    string `yaml:"name"`
	Comment string `yaml:"comment"` // extra field for example purposes
	Email   string `yaml:"email"`
	Entity  *openpgp.Entity
	Admin   *openpgp.Entity
}

// NewGpgRes is a constructor for this resource. It also calls Init() for you.
func NewGpgRes(name string, email string, adminPublicPath string) *GpgRes {
	log.Println("PGP: init PGP")
	obj := &GpgRes{
		Name:    name,
		Comment: "",
		Email:   email,
	}
	obj.Init(adminPublicPath)
	return obj
}

// Init runs some startup code for this resource.
func (obj *GpgRes) Init(adminPublicPath string) {
	log.Println("TESTING PGP")

	var err error
	var config packet.Config
	config.DefaultHash = crypto.SHA256

	obj.Entity, err = openpgp.NewEntity(obj.Name, obj.Comment, obj.Email, &config)
	if err != nil {
		log.Println(err)
		return
	}

	// Self Sign for futur export
	obj.SelfSign()

	obj.Admin = addAdminPubKey(adminPublicPath)
}

// SavePubKey save the public key of an entity for later import
func (obj *GpgRes) SavePubKey(prefix string) {
	log.Println("PGP: Save Public key")

	file := prefix + "/PubGPG1.gpg"

	f, err := os.Create(file)
	checkError(err)
	w := bufio.NewWriter(f)

	obj.Entity.Serialize(w)

	// buf.WriteTo(w)
	w.Flush()

	log.Println("Create File")

}

// addAdminPubKey Allow to encrypt message for admin
func addAdminPubKey(path string) *openpgp.Entity {
	// Read in public key
	log.Println("PGP: Admin pub Key file")
	pubKeyFile, _ := os.Open(path)
	defer pubKeyFile.Close()

	file := packet.NewReader(bufio.NewReader(pubKeyFile))

	entity, err := openpgp.ReadEntity(file)
	checkError(err)

	log.Println(entity)
	return entity
}

// SelfSign Allow Serialization and Export of public key
// REF : https://github.com/alokmenghrajani/gpgeez/blob/master/gpgeez.go
func (obj *GpgRes) SelfSign() {
	var config packet.Config
	config.DefaultHash = crypto.SHA256

	log.Println("Sign Entity")

	key := obj.Entity
	for _, id := range key.Identities {
		id.SelfSignature.PreferredSymmetric = []uint8{
			uint8(packet.CipherAES256),
			uint8(packet.CipherAES192),
			uint8(packet.CipherAES128),
			uint8(packet.CipherCAST5),
			uint8(packet.Cipher3DES),
		}
		id.SelfSignature.PreferredHash = []uint8{
			uint8(crypto.SHA256),
			uint8(crypto.SHA1),
			uint8(crypto.SHA384),
			uint8(crypto.SHA512),
			uint8(crypto.SHA224),
		}
		id.SelfSignature.PreferredCompression = []uint8{
			uint8(packet.CompressionZLIB),
			uint8(packet.CompressionZIP),
		}

		id.SelfSignature.SignUserId(id.UserId.Id, key.PrimaryKey, key.PrivateKey, &config)
	}

	// Self-sign the Subkeys
	for _, subkey := range key.Subkeys {
		subkey.Sig.SignKey(subkey.PublicKey, key.PrivateKey, &config)
	}

}

// Crypt encode the encrypted string from CryptingMsg
func (obj *GpgRes) Crypt(to *openpgp.Entity, msg string) string {
	// Crypting
	buf := obj.CryptingMsg(to, msg)

	// Encode to base64
	bytes, err := ioutil.ReadAll(buf)
	checkError(err)
	encString := base64.StdEncoding.EncodeToString(bytes)
	// Output encrypted/encoded string
	log.Println("Encrypted Secret:", encString)

	return encString
}

// CryptingMsg encrypt the message.
func (obj *GpgRes) CryptingMsg(to *openpgp.Entity, msg string) *bytes.Buffer {
	ents := []*openpgp.Entity{to}

	log.Println("PGP: Crypting the test file")

	buf := new(bytes.Buffer)
	w, err := openpgp.Encrypt(buf, ents, obj.Entity, nil, nil)
	checkError(err)

	_, err = w.Write([]byte(msg))
	checkError(err)

	err = w.Close()
	checkError(err)
	return buf
}

// WriteToAdmin write an encrypted message in a file
func (obj *GpgRes) WriteToAdmin(msg string, prefix string) {
	log.Println("PGP: Writing messages to Admin")
	file := prefix + "/MessageForAdmin.gpg"

	buf := obj.CryptingMsg(obj.Admin, msg)

	f, err := os.Create(file)
	checkError(err)
	w := bufio.NewWriter(f)

	buf.WriteTo(w)
	w.Flush()

	log.Println("Create File")
}

// Decrypt a encrypted msg
func (obj *GpgRes) Decrypt(encString string) string {
	entityList := openpgp.EntityList{obj.Entity}
	log.Println("Decrypting the test file")

	// Decode the base64 string
	dec, err := base64.StdEncoding.DecodeString(encString)
	checkError(err)

	// Decrypt it with the contents of the private key
	md, err := openpgp.ReadMessage(bytes.NewBuffer(dec), entityList, nil, nil)
	checkError(err)

	bytes, err := ioutil.ReadAll(md.UnverifiedBody)
	checkError(err)

	return string(bytes)
}

func checkError(err error) {
	if err != nil {
		log.Println(err)
	}
}
